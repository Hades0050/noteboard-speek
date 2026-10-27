use std::path::PathBuf;
use std::sync::mpsc::{channel, Sender};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread;

use noteboard_core::stt::{Model, Stt};
use noteboard_core::{rpc, App, Error};
use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine;
use serde::Serialize;
use tauri::ipc::{Channel, InvokeBody, Request, Response};
use tauri::{Manager, State};

struct Ctx {
    app: Arc<App>,
    model: Arc<Model>,
    wake: Sender<()>,
}

#[derive(Serialize)]
struct RpcError {
    code: String,
    message: String,
}

fn to_rpc_error(e: Error) -> RpcError {
    let code = match e {
        Error::NotFound => "not_found",
        Error::Invalid(_) => "invalid_argument",
        Error::Other(_) => "internal",
    };
    RpcError { code: code.into(), message: e.to_string() }
}

/// На Android IPC идёт только через postMessage: там нет сырого тела запроса,
/// и байты доехали бы массивом чисел. Поэтому фронт всегда шлёт base64.
fn bytes(request: &Request<'_>) -> Result<Vec<u8>, RpcError> {
    let InvokeBody::Json(serde_json::Value::String(data)) = request.body() else {
        return Err(RpcError { code: "invalid_argument".into(), message: "ожидался base64".into() });
    };
    BASE64
        .decode(data)
        .map_err(|e| RpcError { code: "invalid_argument".into(), message: e.to_string() })
}

#[tauri::command]
fn rpc(request: Request<'_>, ctx: State<'_, Ctx>) -> Result<Response, RpcError> {
    let body = bytes(&request)?;
    let method = request
        .headers()
        .get("method")
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default()
        .to_string();

    rpc::dispatch(&ctx.app, &method, &body)
        .map(Response::new)
        .map_err(to_rpc_error)
}

/// Аудио приходит уже сконвертированным в WAV 16 kHz mono: в WebView это
/// делает AudioContext, в Rust нет декодера Opus.
#[tauri::command]
fn attach_audio(request: Request<'_>, ctx: State<'_, Ctx>) -> Result<Response, RpcError> {
    let body = bytes(&request)?;
    let id: i64 = request
        .headers()
        .get("note-id")
        .and_then(|v| v.to_str().ok())
        .and_then(|v| v.parse().ok())
        .unwrap_or(0);

    let note = ctx.app.attach_audio(id, &body, "wav").map_err(to_rpc_error)?;
    let _ = ctx.wake.send(());
    Ok(Response::new(
        serde_json::to_vec(&serde_json::json!({ "id": note.id, "audio_path": note.audio_path }))
            .unwrap_or_default(),
    ))
}

#[tauri::command]
fn upload_image(request: Request<'_>, ctx: State<'_, Ctx>) -> Result<String, RpcError> {
    let body = bytes(&request)?;
    let ext = request
        .headers()
        .get("ext")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("png")
        .to_string();
    let name = format!("image-{}.{ext}", noteboard_core::now().replace(':', "-"));
    let path = ctx.app.media_path(&name);
    std::fs::write(&path, &body)
        .map_err(|e| RpcError { code: "internal".into(), message: e.to_string() })?;
    Ok(path.to_string_lossy().into_owned())
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ModelProgress {
    file: String,
    done: u64,
    total: u64,
}

#[tauri::command]
fn model_ready(ctx: State<'_, Ctx>) -> bool {
    ctx.model.ready()
}

#[tauri::command]
async fn model_download(ctx: State<'_, Ctx>, progress: Channel<ModelProgress>) -> Result<(), RpcError> {
    let model = ctx.model.clone();
    tauri::async_runtime::spawn_blocking(move || {
        model.download(|file, done, total| {
            let _ = progress.send(ModelProgress { file: file.to_string(), done, total });
        })
    })
    .await
    .map_err(|e| RpcError { code: "internal".into(), message: e.to_string() })?
    .map_err(to_rpc_error)
}

/// Воркер расшифровки: один поток, модель грузится один раз и остаётся в памяти.
fn spawn_worker(app: Arc<App>, model: Arc<Model>) -> Sender<()> {
    let (tx, rx) = channel::<()>();
    thread::spawn(move || {
        let stt: OnceLock<Mutex<Stt>> = OnceLock::new();
        loop {
            while let Ok(Some(note)) = app.claim() {
                if !model.ready() {
                    let _ = app.fail(note.id, "модель не скачана");
                    continue;
                }
                let loaded = stt.get_or_init(|| {
                    Mutex::new(Stt::load(&model).expect("загрузка модели"))
                });
                let result = loaded
                    .lock()
                    .expect("stt mutex")
                    .transcribe(std::path::Path::new(&note.audio_path));
                match result {
                    Ok(t) => {
                        let _ = app.finish(note.id, &t.text);
                    }
                    Err(e) => {
                        let _ = app.fail(note.id, &e.to_string());
                    }
                }
            }
            if rx.recv().is_err() {
                return;
            }
        }
    });
    tx
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|handle| {
            let data_dir: PathBuf = handle.path().app_data_dir()?;
            let app = Arc::new(App::open(data_dir.clone())?);
            let model = Arc::new(Model::new(data_dir.join("model")));
            let wake = spawn_worker(app.clone(), model.clone());
            let _ = wake.send(()); // подобрать всё, что осталось со прошлого запуска
            handle.manage(Ctx { app, model, wake });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            rpc,
            attach_audio,
            upload_image,
            model_ready,
            model_download
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
