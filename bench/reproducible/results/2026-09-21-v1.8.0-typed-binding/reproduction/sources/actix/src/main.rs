use actix_web::{
    body::MessageBody,
    dev::{ServiceRequest, ServiceResponse},
    error::ErrorBadRequest,
    http::header::{HeaderName, HeaderValue},
    middleware::{from_fn, Next},
    web, App, Error, HttpMessage, HttpResponse, HttpServer,
};
use serde::{de, Deserialize, Deserializer, Serialize};
use std::{env, fmt, io, str::FromStr};

fn default_id() -> i64 {
    7
}

fn default_page() -> i64 {
    1
}

fn default_active() -> bool {
    true
}

fn default_score() -> f64 {
    1.5
}

fn default_name() -> String {
    "guest".to_owned()
}

// Empty form values retain defaults, matching the fixture's binding contract.
fn deserialize_scalar<'de, D, T>(deserializer: D, default: T) -> Result<T, D::Error>
where
    D: Deserializer<'de>,
    T: FromStr,
    T::Err: fmt::Display,
{
    struct Scalar<T>(T);

    impl<'de, T> de::Visitor<'de> for Scalar<T>
    where
        T: FromStr,
        T::Err: fmt::Display,
    {
        type Value = T;

        fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
            formatter.write_str("a scalar query value")
        }

        fn visit_str<E: de::Error>(self, value: &str) -> Result<T, E> {
            if value.is_empty() {
                Ok(self.0)
            } else {
                value.parse().map_err(E::custom)
            }
        }
    }

    deserializer.deserialize_str(Scalar(default))
}

fn deserialize_id<'de, D: Deserializer<'de>>(deserializer: D) -> Result<i64, D::Error> {
    deserialize_scalar(deserializer, default_id())
}

fn deserialize_page<'de, D: Deserializer<'de>>(deserializer: D) -> Result<i64, D::Error> {
    deserialize_scalar(deserializer, default_page())
}

struct QueryBool(bool);

impl FromStr for QueryBool {
    type Err = &'static str;

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        match value {
            "1" | "t" | "T" | "TRUE" | "true" | "True" => Ok(Self(true)),
            "0" | "f" | "F" | "FALSE" | "false" | "False" => Ok(Self(false)),
            _ => Err("invalid boolean query value"),
        }
    }
}

fn deserialize_active<'de, D: Deserializer<'de>>(deserializer: D) -> Result<bool, D::Error> {
    deserialize_scalar(deserializer, QueryBool(default_active())).map(|value| value.0)
}

fn deserialize_score<'de, D: Deserializer<'de>>(deserializer: D) -> Result<f64, D::Error> {
    deserialize_scalar(deserializer, default_score())
}

fn deserialize_name<'de, D: Deserializer<'de>>(deserializer: D) -> Result<String, D::Error> {
    let name = String::deserialize(deserializer)?;
    Ok(if name.is_empty() {
        default_name()
    } else {
        name
    })
}


#[derive(Deserialize)]
struct UserPath { id: i64 }

#[derive(Serialize)]
struct FieldError {
    field: &'static str,
    rule: &'static str,
    param: &'static str,
    message: String,
    value: String,
}

#[derive(Serialize)]
struct ValidationResponse {
    code: u16,
    message: &'static str,
    errors: Vec<FieldError>,
}

fn failure(field: &'static str, rule: &'static str, param: &'static str, value: impl fmt::Display) -> FieldError {
    let bound = if rule == "min" { "least" } else { "most" };
    FieldError { field, rule, param, message: format!("{field} must be at {bound} {param}"), value: value.to_string() }
}

#[derive(Deserialize, Serialize)]
struct Input5 {
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID"))]
    id: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page"))]
    page: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active"))]
    active: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score"))]
    score: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name"))]
    name: String,
}

async fn typed_5(query: web::Query<Input5>, path: web::Path<UserPath>) -> HttpResponse {
    let mut input = query.into_inner();
    input.id = path.id;
    let trimmed = input.name.trim();
    if trimmed.len() != input.name.len() { input.name = trimmed.to_owned(); }
    let mut errors = Vec::new();
    if !(input.id >= 1) { errors.push(failure("id", "min", "1", &input.id)); }
    if !(input.page >= 1) { errors.push(failure("page", "min", "1", &input.page)); }
    if !(input.page <= 1000) { errors.push(failure("page", "max", "1000", &input.page)); }
    if !(input.score >= 0.0) { errors.push(failure("score", "min", "0", &input.score)); }
    if !(input.score <= 100.0) { errors.push(failure("score", "max", "100", &input.score)); }
    if !(input.name.len() >= 1) { errors.push(failure("name", "min", "1", &input.name)); }
    if !(input.name.len() <= 64) { errors.push(failure("name", "max", "64", &input.name)); }
    if !errors.is_empty() {
        return HttpResponse::UnprocessableEntity().content_type("application/json; charset=utf-8").json(ValidationResponse { code: 422, message: "Validation failed", errors });
    }
    HttpResponse::Ok().content_type("application/json; charset=utf-8").json(input)
}

#[derive(Deserialize, Serialize)]
struct Input10 {
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID"))]
    id: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page"))]
    page: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active"))]
    active: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score"))]
    score: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name"))]
    name: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID2"))]
    id2: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page2"))]
    page2: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active2"))]
    active2: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score2"))]
    score2: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name2"))]
    name2: String,
}

async fn typed_10(query: web::Query<Input10>, path: web::Path<UserPath>) -> HttpResponse {
    let mut input = query.into_inner();
    input.id = path.id;
    let trimmed = input.name.trim();
    if trimmed.len() != input.name.len() { input.name = trimmed.to_owned(); }
    let trimmed = input.name2.trim();
    if trimmed.len() != input.name2.len() { input.name2 = trimmed.to_owned(); }
    let mut errors = Vec::new();
    if !(input.id >= 1) { errors.push(failure("id", "min", "1", &input.id)); }
    if !(input.page >= 1) { errors.push(failure("page", "min", "1", &input.page)); }
    if !(input.page <= 1000) { errors.push(failure("page", "max", "1000", &input.page)); }
    if !(input.score >= 0.0) { errors.push(failure("score", "min", "0", &input.score)); }
    if !(input.score <= 100.0) { errors.push(failure("score", "max", "100", &input.score)); }
    if !(input.name.len() >= 1) { errors.push(failure("name", "min", "1", &input.name)); }
    if !(input.name.len() <= 64) { errors.push(failure("name", "max", "64", &input.name)); }
    if !(input.id2 >= 1) { errors.push(failure("id2", "min", "1", &input.id2)); }
    if !(input.page2 >= 1) { errors.push(failure("page2", "min", "1", &input.page2)); }
    if !(input.page2 <= 1000) { errors.push(failure("page2", "max", "1000", &input.page2)); }
    if !(input.score2 >= 0.0) { errors.push(failure("score2", "min", "0", &input.score2)); }
    if !(input.score2 <= 100.0) { errors.push(failure("score2", "max", "100", &input.score2)); }
    if !(input.name2.len() >= 1) { errors.push(failure("name2", "min", "1", &input.name2)); }
    if !(input.name2.len() <= 64) { errors.push(failure("name2", "max", "64", &input.name2)); }
    if !errors.is_empty() {
        return HttpResponse::UnprocessableEntity().content_type("application/json; charset=utf-8").json(ValidationResponse { code: 422, message: "Validation failed", errors });
    }
    HttpResponse::Ok().content_type("application/json; charset=utf-8").json(input)
}

#[derive(Deserialize, Serialize)]
struct Input30 {
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID"))]
    id: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page"))]
    page: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active"))]
    active: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score"))]
    score: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name"))]
    name: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID2"))]
    id2: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page2"))]
    page2: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active2"))]
    active2: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score2"))]
    score2: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name2"))]
    name2: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID3"))]
    id3: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page3"))]
    page3: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active3"))]
    active3: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score3"))]
    score3: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name3"))]
    name3: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID4"))]
    id4: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page4"))]
    page4: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active4"))]
    active4: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score4"))]
    score4: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name4"))]
    name4: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID5"))]
    id5: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page5"))]
    page5: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active5"))]
    active5: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score5"))]
    score5: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name5"))]
    name5: String,
    #[serde(default = "default_id", deserialize_with = "deserialize_id", rename(serialize = "ID6"))]
    id6: i64,
    #[serde(default = "default_page", deserialize_with = "deserialize_page", rename(serialize = "Page6"))]
    page6: i64,
    #[serde(default = "default_active", deserialize_with = "deserialize_active", rename(serialize = "Active6"))]
    active6: bool,
    #[serde(default = "default_score", deserialize_with = "deserialize_score", rename(serialize = "Score6"))]
    score6: f64,
    #[serde(default = "default_name", deserialize_with = "deserialize_name", rename(serialize = "Name6"))]
    name6: String,
}

async fn typed_30(query: web::Query<Input30>, path: web::Path<UserPath>) -> HttpResponse {
    let mut input = query.into_inner();
    input.id = path.id;
    let trimmed = input.name.trim();
    if trimmed.len() != input.name.len() { input.name = trimmed.to_owned(); }
    let trimmed = input.name2.trim();
    if trimmed.len() != input.name2.len() { input.name2 = trimmed.to_owned(); }
    let trimmed = input.name3.trim();
    if trimmed.len() != input.name3.len() { input.name3 = trimmed.to_owned(); }
    let trimmed = input.name4.trim();
    if trimmed.len() != input.name4.len() { input.name4 = trimmed.to_owned(); }
    let trimmed = input.name5.trim();
    if trimmed.len() != input.name5.len() { input.name5 = trimmed.to_owned(); }
    let trimmed = input.name6.trim();
    if trimmed.len() != input.name6.len() { input.name6 = trimmed.to_owned(); }
    let mut errors = Vec::new();
    if !(input.id >= 1) { errors.push(failure("id", "min", "1", &input.id)); }
    if !(input.page >= 1) { errors.push(failure("page", "min", "1", &input.page)); }
    if !(input.page <= 1000) { errors.push(failure("page", "max", "1000", &input.page)); }
    if !(input.score >= 0.0) { errors.push(failure("score", "min", "0", &input.score)); }
    if !(input.score <= 100.0) { errors.push(failure("score", "max", "100", &input.score)); }
    if !(input.name.len() >= 1) { errors.push(failure("name", "min", "1", &input.name)); }
    if !(input.name.len() <= 64) { errors.push(failure("name", "max", "64", &input.name)); }
    if !(input.id2 >= 1) { errors.push(failure("id2", "min", "1", &input.id2)); }
    if !(input.page2 >= 1) { errors.push(failure("page2", "min", "1", &input.page2)); }
    if !(input.page2 <= 1000) { errors.push(failure("page2", "max", "1000", &input.page2)); }
    if !(input.score2 >= 0.0) { errors.push(failure("score2", "min", "0", &input.score2)); }
    if !(input.score2 <= 100.0) { errors.push(failure("score2", "max", "100", &input.score2)); }
    if !(input.name2.len() >= 1) { errors.push(failure("name2", "min", "1", &input.name2)); }
    if !(input.name2.len() <= 64) { errors.push(failure("name2", "max", "64", &input.name2)); }
    if !(input.id3 >= 1) { errors.push(failure("id3", "min", "1", &input.id3)); }
    if !(input.page3 >= 1) { errors.push(failure("page3", "min", "1", &input.page3)); }
    if !(input.page3 <= 1000) { errors.push(failure("page3", "max", "1000", &input.page3)); }
    if !(input.score3 >= 0.0) { errors.push(failure("score3", "min", "0", &input.score3)); }
    if !(input.score3 <= 100.0) { errors.push(failure("score3", "max", "100", &input.score3)); }
    if !(input.name3.len() >= 1) { errors.push(failure("name3", "min", "1", &input.name3)); }
    if !(input.name3.len() <= 64) { errors.push(failure("name3", "max", "64", &input.name3)); }
    if !(input.id4 >= 1) { errors.push(failure("id4", "min", "1", &input.id4)); }
    if !(input.page4 >= 1) { errors.push(failure("page4", "min", "1", &input.page4)); }
    if !(input.page4 <= 1000) { errors.push(failure("page4", "max", "1000", &input.page4)); }
    if !(input.score4 >= 0.0) { errors.push(failure("score4", "min", "0", &input.score4)); }
    if !(input.score4 <= 100.0) { errors.push(failure("score4", "max", "100", &input.score4)); }
    if !(input.name4.len() >= 1) { errors.push(failure("name4", "min", "1", &input.name4)); }
    if !(input.name4.len() <= 64) { errors.push(failure("name4", "max", "64", &input.name4)); }
    if !(input.id5 >= 1) { errors.push(failure("id5", "min", "1", &input.id5)); }
    if !(input.page5 >= 1) { errors.push(failure("page5", "min", "1", &input.page5)); }
    if !(input.page5 <= 1000) { errors.push(failure("page5", "max", "1000", &input.page5)); }
    if !(input.score5 >= 0.0) { errors.push(failure("score5", "min", "0", &input.score5)); }
    if !(input.score5 <= 100.0) { errors.push(failure("score5", "max", "100", &input.score5)); }
    if !(input.name5.len() >= 1) { errors.push(failure("name5", "min", "1", &input.name5)); }
    if !(input.name5.len() <= 64) { errors.push(failure("name5", "max", "64", &input.name5)); }
    if !(input.id6 >= 1) { errors.push(failure("id6", "min", "1", &input.id6)); }
    if !(input.page6 >= 1) { errors.push(failure("page6", "min", "1", &input.page6)); }
    if !(input.page6 <= 1000) { errors.push(failure("page6", "max", "1000", &input.page6)); }
    if !(input.score6 >= 0.0) { errors.push(failure("score6", "min", "0", &input.score6)); }
    if !(input.score6 <= 100.0) { errors.push(failure("score6", "max", "100", &input.score6)); }
    if !(input.name6.len() >= 1) { errors.push(failure("name6", "min", "1", &input.name6)); }
    if !(input.name6.len() <= 64) { errors.push(failure("name6", "max", "64", &input.name6)); }
    if !errors.is_empty() {
        return HttpResponse::UnprocessableEntity().content_type("application/json; charset=utf-8").json(ValidationResponse { code: 422, message: "Validation failed", errors });
    }
    HttpResponse::Ok().content_type("application/json; charset=utf-8").json(input)
}

async fn request_marker(
    request: ServiceRequest,
    next: Next<impl MessageBody>,
) -> Result<ServiceResponse<impl MessageBody>, Error> {
    request.extensions_mut().insert("benchmark");
    next.call(request).await
}

async fn benchmark_header(
    request: ServiceRequest,
    next: Next<impl MessageBody>,
) -> Result<ServiceResponse<impl MessageBody>, Error> {
    let mut response = next.call(request).await?;
    response.headers_mut().insert(
        HeaderName::from_static("x-benchmark"),
        HeaderValue::from_static("bindgen"),
    );
    Ok(response)
}

#[actix_web::main]
async fn main() -> io::Result<()> {
    let fields = env::var("BENCH_FIELDS").unwrap_or_default();
    if !matches!(fields.as_str(), "5" | "10" | "30") {
        return Err(io::Error::new(io::ErrorKind::InvalidInput, "BENCH_FIELDS must be 5, 10 or 30"));
    }
    let workers_raw = env::var("BENCH_WORKERS").unwrap_or_default();
    if !matches!(workers_raw.as_str(), "4" | "8") {
        return Err(io::Error::new(io::ErrorKind::InvalidInput, "BENCH_WORKERS must be 4 or 8"));
    }
    let workers = workers_raw.parse::<usize>().unwrap();
    let port = env::var("PORT").unwrap_or_else(|_| "18373".to_owned());
    let addr = format!("127.0.0.1:{port}");
    println!("actix=4.15.0 fields={fields} workers={workers} addr={addr}");
    HttpServer::new(move || {
        let route = match fields.as_str() {
            "5" => web::get().to(typed_5),
            "10" => web::get().to(typed_10),
            _ => web::get().to(typed_30),
        };
        App::new()
        .app_data(web::PathConfig::default().error_handler(|error, _| ErrorBadRequest(error)))
        .service(
            web::resource("/users/{id}")
                .wrap(from_fn(benchmark_header))
                .wrap(from_fn(request_marker))
                .route(route),
        )
    })
    .workers(workers)
    .shutdown_timeout(5)
    .bind(addr)?
    .run()
    .await
}
