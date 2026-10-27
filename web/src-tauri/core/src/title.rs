use crate::db::Note;

/// Сколько символов заголовка помещается на карточке.
const TITLE_CHARS: usize = 60;

/// Подпись карточки: заголовок заметки, иначе начало текста, иначе начало
/// расшифровки. Тело хранится как HTML из редактора, поэтому теги и сущности
/// сначала убираются.
pub fn card_title(n: &Note) -> String {
    for candidate in [n.title.clone(), excerpt(&n.body), excerpt(&n.transcript)] {
        if !candidate.is_empty() {
            return candidate;
        }
    }
    "Заметка".into()
}

pub fn excerpt(s: &str) -> String {
    let stripped = strip_tags(s);
    let text = unescape(&stripped);
    let text = text.split_whitespace().collect::<Vec<_>>().join(" ");
    let chars: Vec<char> = text.chars().collect();
    if chars.len() > TITLE_CHARS {
        let head: String = chars[..TITLE_CHARS].iter().collect();
        return format!("{}…", head.trim_end());
    }
    text
}

fn strip_tags(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut depth = 0usize;
    for ch in s.chars() {
        match ch {
            '<' => {
                depth += 1;
                out.push(' ');
            }
            '>' if depth > 0 => depth -= 1,
            _ if depth == 0 => out.push(ch),
            _ => {}
        }
    }
    out
}

fn unescape(s: &str) -> String {
    s.replace("&nbsp;", " ")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", "\"")
        .replace("&#39;", "'")
        .replace("&amp;", "&")
}
