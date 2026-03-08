use actix_web::{web, App, HttpResponse, HttpServer};
use deadpool_postgres::{Config, Pool, Runtime};
use rand::Rng;
use serde::{Deserialize, Serialize};
use tokio_postgres::NoTls;

#[derive(Serialize, Deserialize)]
struct JsonBody {
    name: String,
    value: f64,
}

#[derive(Serialize)]
struct World {
    id: i32,
    #[serde(rename = "randomNumber")]
    random_number: i32,
}

struct Fortune {
    id: i32,
    message: String,
}

async fn plaintext() -> HttpResponse {
    HttpResponse::Ok()
        .content_type("text/plain")
        .body("Hello, World!")
}

async fn json_get() -> HttpResponse {
    HttpResponse::Ok()
        .content_type("application/json")
        .body(r#"{"message":"Hello, World!"}"#)
}

async fn param(path: web::Path<String>) -> HttpResponse {
    HttpResponse::Ok()
        .content_type("application/json")
        .body(format!("{{\"id\":\"{}\"}}", path.into_inner()))
}

async fn json_post(body: web::Json<JsonBody>) -> HttpResponse {
    HttpResponse::Ok().json(&*body)
}

async fn db(pool: web::Data<Pool>) -> HttpResponse {
    let client = pool.get().await.unwrap();
    let id = rand::thread_rng().gen_range(1..=10000);
    let row = client
        .query_one("SELECT id, randomnumber FROM world WHERE id=$1", &[&id])
        .await
        .unwrap();
    let w = World {
        id: row.get(0),
        random_number: row.get(1),
    };
    HttpResponse::Ok().json(w)
}

async fn fortunes(pool: web::Data<Pool>) -> HttpResponse {
    let client = pool.get().await.unwrap();
    let rows = client
        .query("SELECT id, message FROM fortune", &[])
        .await
        .unwrap();
    let mut fortunes: Vec<Fortune> = rows
        .iter()
        .map(|r| Fortune {
            id: r.get(0),
            message: r.get(1),
        })
        .collect();
    fortunes.push(Fortune {
        id: 0,
        message: "Additional fortune added at request time.".to_string(),
    });
    fortunes.sort_by(|a, b| a.message.cmp(&b.message));

    let mut buf = String::with_capacity(2048);
    buf.push_str("<!DOCTYPE html><html><head><title>Fortunes</title></head><body><table><tr><th>id</th><th>message</th></tr>");
    for f in &fortunes {
        buf.push_str("<tr><td>");
        buf.push_str(&f.id.to_string());
        buf.push_str("</td><td>");
        buf.push_str(&html_escape(&f.message));
        buf.push_str("</td></tr>");
    }
    buf.push_str("</table></body></html>");
    HttpResponse::Ok()
        .content_type("text/html; charset=utf-8")
        .body(buf)
}

async fn updates(pool: web::Data<Pool>, query: web::Query<std::collections::HashMap<String, String>>) -> HttpResponse {
    let n: i32 = query.get("q").and_then(|v| v.parse().ok()).unwrap_or(1).max(1).min(500);
    let client = pool.get().await.unwrap();
    let mut worlds = Vec::with_capacity(n as usize);
    let mut rng = rand::thread_rng();
    for _ in 0..n {
        let id: i32 = rng.gen_range(1..=10000);
        let row = client
            .query_one("SELECT id, randomnumber FROM world WHERE id=$1", &[&id])
            .await
            .unwrap();
        worlds.push(World {
            id: row.get(0),
            random_number: rng.gen_range(1..=10000),
        });
    }
    // Batch update using unnest
    let ids: Vec<i32> = worlds.iter().map(|w| w.id).collect();
    let nums: Vec<i32> = worlds.iter().map(|w| w.random_number).collect();
    client
        .execute(
            "UPDATE world SET randomnumber=v.r FROM (SELECT unnest($1::int[]) id, unnest($2::int[]) r) v WHERE world.id=v.id",
            &[&ids, &nums],
        )
        .await
        .unwrap();
    HttpResponse::Ok().json(worlds)
}

fn html_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&#x27;")
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let port: u16 = std::env::var("PORT")
        .unwrap_or_else(|_| "3003".to_string())
        .parse()
        .unwrap_or(3003);

    let dsn = std::env::var("DATABASE_URL")
        .unwrap_or_else(|_| "postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world".to_string());

    let mut cfg = Config::new();
    cfg.url = Some(dsn);
    cfg.pool = Some(deadpool_postgres::PoolConfig::new(64));
    let pool = cfg.create_pool(Some(Runtime::Tokio1), NoTls).unwrap();

    let pool_data = web::Data::new(pool);

    HttpServer::new(move || {
        App::new()
            .app_data(pool_data.clone())
            .route("/", web::get().to(plaintext))
            .route("/json", web::get().to(json_get))
            .route("/users/{id}", web::get().to(param))
            .route("/json", web::post().to(json_post))
            .route("/db", web::get().to(db))
            .route("/fortunes", web::get().to(fortunes))
            .route("/updates", web::get().to(updates))
    })
    .bind(("0.0.0.0", port))?
    .run()
    .await
}
