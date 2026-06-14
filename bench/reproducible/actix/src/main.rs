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
struct JsonMessage {
    message: &'static str,
}

#[derive(Serialize)]
struct World {
    id: i32,
    #[serde(rename = "randomNumber")]
    random_number: i32,
}

#[derive(Serialize)]
struct RealworldProfileResponse {
    #[serde(rename = "requestId")]
    request_id: String,
    profile: RealworldProfile,
    summary: RealworldProfileSummary,
}

#[derive(Serialize)]
struct RealworldProfile {
    id: i32,
    name: &'static str,
    email: &'static str,
    #[serde(rename = "randomNumber")]
    random_number: i32,
    include: String,
}

#[derive(Serialize)]
struct RealworldProfileSummary {
    limit: i32,
    features: [&'static str; 4],
    #[serde(rename = "traceMode")]
    trace_mode: &'static str,
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

async fn json_static() -> HttpResponse {
    HttpResponse::Ok()
        .content_type("application/json")
        .body(r#"{"message":"Hello, World!"}"#)
}

async fn json_serialize() -> HttpResponse {
    HttpResponse::Ok().json(JsonMessage {
        message: "Hello, World!",
    })
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

async fn queries(
    pool: web::Data<Pool>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> HttpResponse {
    let n: i32 = query
        .get("q")
        .and_then(|v| v.parse().ok())
        .unwrap_or(1)
        .max(1)
        .min(500);
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
            random_number: row.get(1),
        });
    }
    HttpResponse::Ok().json(worlds)
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

async fn updates(
    pool: web::Data<Pool>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> HttpResponse {
    let n: i32 = query
        .get("q")
        .and_then(|v| v.parse().ok())
        .unwrap_or(1)
        .max(1)
        .min(500);
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

async fn realworld_profile(
    req: actix_web::HttpRequest,
    pool: web::Data<Pool>,
    path: web::Path<i32>,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> HttpResponse {
    if query.get("token").map(|v| v.as_str()) != Some("benchmark-token") {
        return HttpResponse::Unauthorized().json(serde_json::json!({"error": "unauthorized"}));
    }

    let id = path.into_inner();
    if !(1..=10000).contains(&id) {
        return HttpResponse::BadRequest().json(serde_json::json!({"error": "invalid profile id"}));
    }

    let include = query
        .get("include")
        .cloned()
        .unwrap_or_else(|| "summary".to_string());
    if include != "summary" && include != "detail" {
        return HttpResponse::BadRequest().json(serde_json::json!({"error": "invalid include"}));
    }

    let limit = query
        .get("limit")
        .and_then(|v| v.parse::<i32>().ok())
        .unwrap_or(3)
        .max(1)
        .min(20);

    let request_id = req
        .headers()
        .get("X-Request-Id")
        .and_then(|v| v.to_str().ok())
        .map(|v| v.to_string())
        .unwrap_or_else(|| format!("bench-{id}"));

    let client = pool.get().await.unwrap();
    let row = client
        .query_one("SELECT randomnumber FROM world WHERE id=$1", &[&id])
        .await
        .unwrap();
    let random_number: i32 = row.get(0);

    HttpResponse::Ok()
        .insert_header(("X-Request-Id", request_id.clone()))
        .json(RealworldProfileResponse {
            request_id,
            profile: RealworldProfile {
                id,
                name: "Tiger Team",
                email: "tiger@kruda.dev",
                random_number,
                include,
            },
            summary: RealworldProfileSummary {
                limit,
                features: ["auth", "validation", "postgres", "json"],
                trace_mode: "request-id",
            },
        })
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

    let enable_db = std::env::var("BENCH_ENABLE_DB").ok().as_deref() == Some("1");
    let workers = std::env::var("BENCH_ACTIX_WORKERS").ok().and_then(|raw| {
        if raw.is_empty() {
            return None;
        }
        let value = raw
            .parse::<usize>()
            .unwrap_or_else(|_| panic!("BENCH_ACTIX_WORKERS must be a positive integer: {raw}"));
        assert!(
            value > 0,
            "BENCH_ACTIX_WORKERS must be a positive integer: {raw}"
        );
        Some(value)
    });
    let pool_data = if enable_db {
        let dsn = std::env::var("DATABASE_URL").unwrap_or_else(|_| {
            "postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world".to_string()
        });

        let mut cfg = Config::new();
        cfg.url = Some(dsn);
        cfg.pool = Some(deadpool_postgres::PoolConfig::new(64));
        Some(web::Data::new(
            cfg.create_pool(Some(Runtime::Tokio1), NoTls).unwrap(),
        ))
    } else {
        None
    };

    let server = HttpServer::new(move || {
        let app = App::new()
            .route("/", web::get().to(plaintext))
            .route("/plaintext-handler", web::get().to(plaintext))
            .route("/json", web::get().to(json_serialize))
            .route("/json-static", web::get().to(json_static))
            .route("/json-serialize", web::get().to(json_serialize))
            .route("/users/{id}", web::get().to(param))
            .route("/json", web::post().to(json_post));

        if let Some(pool) = &pool_data {
            app.app_data(pool.clone())
                .route("/db", web::get().to(db))
                .route("/queries", web::get().to(queries))
                .route("/fortunes", web::get().to(fortunes))
                .route("/updates", web::get().to(updates))
                .route("/realworld-profile/{id}", web::get().to(realworld_profile))
        } else {
            app
        }
    });
    let server = if let Some(workers) = workers {
        server.workers(workers)
    } else {
        server
    };

    server.bind(("0.0.0.0", port))?.run().await
}
