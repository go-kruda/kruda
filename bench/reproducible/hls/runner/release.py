import os
import subprocess

from long import free_port, ready, smoke, stop, verify_reuse


def main():
    origin_address = f"127.0.0.1:{free_port()}"
    proxy_address = f"127.0.0.1:{free_port()}"
    origin = f"http://{origin_address}"
    proxy = f"http://{proxy_address}"
    env = {**os.environ, "GOMAXPROCS": "2"}

    origin_process = subprocess.Popen(["./go-bench", "origin", origin_address], env=env)
    try:
        ready(origin + "/ready")
        proxy_process = subprocess.Popen(["./go-bench", "wing", proxy_address, origin], env=env)
        try:
            smoke(origin, proxy)
            verify_reuse(proxy_address, origin)
        finally:
            stop(proxy_process)
    finally:
        stop(origin_process)

    print("PASS: v1.9.0 GET and Range match origin bytes, HEAD has no body, and GET/Range/GET reused one TCP socket")


if __name__ == "__main__":
    main()
