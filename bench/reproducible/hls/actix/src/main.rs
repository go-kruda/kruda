use actix_web::{App, HttpRequest, HttpResponse, HttpServer, body::SizedStream, web};
use futures_util::{StreamExt, stream};
use actix_web::web::Bytes;
use std::{env, time::Duration};

const HEADERS: [&str; 8] = ["content-type", "content-length", "content-range", "accept-ranges",
    "x-origin-request", "cache-control", "etag", "last-modified"];
struct State { origin: String, client: reqwest::Client, split: usize }

async fn proxy(request: HttpRequest, state: web::Data<State>) -> HttpResponse {
    let method = reqwest::Method::from_bytes(request.method().as_str().as_bytes()).unwrap();
    let mut upstream = state.client.request(method, format!("{}{}", state.origin, request.path()));
    if let Some(value) = request.headers().get("range") {
        upstream = upstream.header("range", value.as_bytes());
    }
    let Ok(response) = upstream.send().await else { return HttpResponse::BadGateway().finish() };
    let status = actix_web::http::StatusCode::from_u16(response.status().as_u16()).unwrap();
    let mut output = HttpResponse::build(status);
    for header in HEADERS {
        if let Some(value) = response.headers().get(header) {
            output.insert_header((header, value.to_str().unwrap()));
        }
    }
    let length = response.headers().get("content-length")
        .and_then(|n| n.to_str().ok()?.parse::<u64>().ok());
    let Some(length) = length else { return HttpResponse::BadGateway().finish() };
    let body = response.bytes_stream().map(|chunk| chunk.map_err(actix_web::error::ErrorBadGateway));
    let maximum = state.split;
    if maximum == 0 {
        return output.body(SizedStream::new(length, body));
    }
    let bounded = stream::unfold((Box::pin(body), Bytes::new()), move |(mut upstream, mut pending)| async move {
        while pending.is_empty() {
            match upstream.next().await? {
                Ok(bytes) => pending = bytes,
                Err(error) => return Some((Err(error), (upstream, pending))),
            }
        }
        let part = pending.split_to(pending.len().min(maximum));
        Some((Ok(part), (upstream, pending)))
    });
    output.body(SizedStream::new(length, bounded))
}

fn upstream_client() -> reqwest::Client {
    reqwest::Client::builder()
        .http1_only().pool_max_idle_per_host(1024).pool_idle_timeout(Duration::from_secs(65))
        .timeout(Duration::from_secs(8)).redirect(reqwest::redirect::Policy::none())
        .build().unwrap()
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let mut args = env::args().skip(1);
    let address = args.next().expect("bind address");
    let origin = args.next().expect("loopback origin");
    let scope = args.next().expect("pool scope: shared or worker");
    assert!(scope == "shared" || scope == "worker");
    let split: usize = args.next().expect("max body chunk").parse().unwrap();
    let write_buffer: usize = args.next().expect("Actix write buffer").parse().unwrap();
    let shared = (scope == "shared").then(upstream_client);
    HttpServer::new(move || {
        let state = State { origin: origin.clone(),
            client: shared.clone().unwrap_or_else(upstream_client), split };
        App::new().app_data(web::Data::new(state)).route("/media/{name}", web::route().to(proxy))
    })
        .workers(2).keep_alive(Duration::from_secs(65))
        .h1_write_buffer_size(write_buffer).tcp_nodelay(true).bind(address)?.run().await
}
