use std::fs;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::time::Instant;

use regex::Regex;
use sherpa_onnx::{
    OfflineRecognizer, OfflineRecognizerConfig, OfflineTransducerModelConfig, SileroVadModelConfig,
    VadModelConfig, VoiceActivityDetector, Wave,
};

use crate::{Error, Result};

const HF: &str = "https://huggingface.co/Smirnov75/GigaAM-v3-sherpa-onnx/resolve/main";
const VAD_WINDOW: usize = 512;
const SAMPLE_RATE: i32 = 16000;

/// Файлы модели: GigaAM v3 e2e RNNT (int8) + Silero VAD. Качаются один раз.
pub const FILES: &[(&str, &str)] = &[
    ("encoder.onnx", "gigaam_v3_e2e_rnnt_encoder_int8.onnx"),
    ("decoder.onnx", "gigaam_v3_e2e_rnnt_decoder.onnx"),
    ("joiner.onnx", "gigaam_v3_e2e_rnnt_joint.onnx"),
    ("tokens.txt", "gigaam_v3_e2e_rnnt_tokens.txt"),
];
const VAD_URL: &str =
    "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx";

pub struct Model {
    pub dir: PathBuf,
}

impl Model {
    pub fn new(dir: PathBuf) -> Self {
        Self { dir }
    }

    pub fn path(&self, name: &str) -> PathBuf {
        self.dir.join(name)
    }

    pub fn ready(&self) -> bool {
        FILES
            .iter()
            .map(|(name, _)| *name)
            .chain(["silero_vad.onnx"])
            .all(|name| self.path(name).is_file())
    }

    /// Скачивает недостающие файлы. `progress(имя, скачано, всего)` — для UI.
    pub fn download(&self, mut progress: impl FnMut(&str, u64, u64)) -> Result<()> {
        fs::create_dir_all(&self.dir)?;
        let urls: Vec<(String, String)> = FILES
            .iter()
            .map(|(name, file)| ((*name).to_string(), format!("{HF}/{file}")))
            .chain([("silero_vad.onnx".to_string(), VAD_URL.to_string())])
            .collect();

        for (name, url) in urls {
            let target = self.path(&name);
            if target.is_file() {
                continue;
            }
            // .part + rename: иначе оборванная загрузка выглядит как готовая модель
            let part = target.with_extension("part");
            let resp = ureq::get(&url)
                .call()
                .map_err(|e| Error::Other(format!("{name}: {e}")))?;
            let total: u64 = resp
                .header("content-length")
                .and_then(|v| v.parse().ok())
                .unwrap_or(0);
            let mut reader = resp.into_reader();
            let mut file = fs::File::create(&part)?;
            let mut buf = vec![0u8; 1 << 16];
            let mut done: u64 = 0;
            loop {
                let n = reader.read(&mut buf)?;
                if n == 0 {
                    break;
                }
                std::io::Write::write_all(&mut file, &buf[..n])?;
                done += n as u64;
                progress(&name, done, total);
            }
            drop(file);
            fs::rename(&part, &target)?;
        }
        Ok(())
    }
}

pub struct Transcript {
    pub text: String,
    pub audio_sec: f64,
    pub decode_sec: f64,
}

/// Распознаватель: модель и VAD грузятся один раз и живут в воркере.
pub struct Stt {
    recognizer: OfflineRecognizer,
    vad: VoiceActivityDetector,
    terms: Vec<(Regex, &'static str)>,
}

impl Stt {
    pub fn load(model: &Model) -> Result<Self> {
        let mut config = OfflineRecognizerConfig::default();
        config.model_config.transducer = OfflineTransducerModelConfig {
            encoder: Some(path_str(&model.path("encoder.onnx"))),
            decoder: Some(path_str(&model.path("decoder.onnx"))),
            joiner: Some(path_str(&model.path("joiner.onnx"))),
        };
        config.model_config.tokens = Some(path_str(&model.path("tokens.txt")));
        config.model_config.model_type = Some("nemo_transducer".into());
        config.model_config.num_threads = 4;
        config.decoding_method = Some("greedy_search".into());

        let recognizer = OfflineRecognizer::create(&config)
            .ok_or_else(|| Error::Other("не удалось создать распознаватель".into()))?;

        let mut vad_config = VadModelConfig {
            sample_rate: SAMPLE_RATE,
            num_threads: 1,
            ..Default::default()
        };
        vad_config.silero_vad = SileroVadModelConfig {
            model: Some(path_str(&model.path("silero_vad.onnx"))),
            threshold: 0.5,
            min_silence_duration: 0.5,
            min_speech_duration: 0.25,
            window_size: VAD_WINDOW as i32,
            // GigaAM принимает не больше ~25 с за раз
            max_speech_duration: 20.0,
        };
        let vad = VoiceActivityDetector::create(&vad_config, 30.0)
            .ok_or_else(|| Error::Other("не удалось создать VAD".into()))?;

        Ok(Self { recognizer, vad, terms: terms() })
    }

    pub fn transcribe(&self, wav: &Path) -> Result<Transcript> {
        let started = Instant::now();
        let wave = Wave::read(&path_str(wav))
            .ok_or_else(|| Error::Other(format!("не читается WAV: {}", wav.display())))?;
        if wave.sample_rate() != SAMPLE_RATE {
            return Err(Error::Invalid(format!(
                "ожидался WAV 16 kHz, получен {} Hz",
                wave.sample_rate()
            )));
        }
        let samples = wave.samples();
        let text = self.recognize(samples);
        Ok(Transcript {
            text: self.fix_terms(&text),
            audio_sec: samples.len() as f64 / SAMPLE_RATE as f64,
            decode_sec: started.elapsed().as_secs_f64(),
        })
    }

    fn recognize(&self, samples: &[f32]) -> String {
        let mut parts: Vec<String> = Vec::new();
        self.vad.reset();
        let mut i = 0;
        while i + VAD_WINDOW <= samples.len() {
            self.vad.accept_waveform(&samples[i..i + VAD_WINDOW]);
            self.drain(&mut parts);
            i += VAD_WINDOW;
        }
        self.vad.flush();
        self.drain(&mut parts);
        parts.join(" ")
    }

    fn drain(&self, parts: &mut Vec<String>) {
        while !self.vad.is_empty() {
            if let Some(segment) = self.vad.front() {
                let stream = self.recognizer.create_stream();
                stream.accept_waveform(SAMPLE_RATE, segment.samples());
                self.recognizer.decode(&stream);
                if let Some(result) = stream.get_result() {
                    let text = result.text.trim().to_string();
                    if !text.is_empty() {
                        parts.push(text);
                    }
                }
            }
            self.vad.pop();
        }
    }

    /// Названия проектов модель пишет кириллицей или по частям — собираем обратно.
    fn fix_terms(&self, text: &str) -> String {
        self.terms.iter().fold(text.to_string(), |acc, (re, name)| {
            re.replace_all(&acc, |c: &regex::Captures<'_>| format!("{}{}", &c[1], name))
                .into_owned()
        })
    }
}

// `_` — граница между частями названия: пробел, дефис или точка от VAD-нарезки.
fn term(core: &str) -> Regex {
    let body = core.replace('_', r"[\s.,-]*");
    Regex::new(&format!(r"(?i)(^|[^а-яёa-zA-ZА-ЯЁ])({body}[а-яёa-z]*)")).expect("term regex")
}

fn terms() -> Vec<(Regex, &'static str)> {
    vec![
        (term(r"(инфо|info)_(к[иеэ]?[оеёэ]?ск|kiosk)"), "InfoKiosk"),
        (term(r"(таск|task)_(компонент|component)"), "TaskComponent"),
        (term(r"(эм_си|мс|mc)_(компонент|component)"), "MCComponent"),
        (term(r"(х[еэи]ль?мет|helmet)"), "Helmet"),
        (term(r"([аэ]пп?|app)_(сет?тинг|sett?[aeiy]n)"), "AppSettings"),
        (term(r"(хоу?м|home)_(п[еэ]йдж|page)"), "HomePage"),
        (term(r"(докер|docker)_(файл|file)_(конвертер|converter)"), "DockerFileConverter"),
    ]
}

fn path_str(p: &Path) -> String {
    p.to_string_lossy().into_owned()
}
