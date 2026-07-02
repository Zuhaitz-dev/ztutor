# ztutor CI image — includes all 7 language toolchains for full sandbox testing.
FROM golang:1.22-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ gdb make \
    python3 \
    ruby \
    openjdk-17-jdk-headless \
    && rm -rf /var/lib/apt/lists/*

# Rust via rustup
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["go", "test", "-v", "-cover", "./internal/..."]
