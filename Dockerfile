FROM golang:1.24-bookworm

WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata gcc g++ libc6-dev curl \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
COPY lqd-sdk-compat/go.mod lqd-sdk-compat/go.sum ./lqd-sdk-compat/
COPY third_party ./third_party

RUN go mod download

COPY . .

RUN test -x scripts/railway/start-chain.sh \
  && test -x scripts/railway/start-signer.sh \
  && test -x scripts/railway/start-wallet.sh \
  && test -x scripts/railway/start-aggregator.sh

RUN sh scripts/railway/build.sh

EXPOSE 6500 6100 8080 9000

CMD ["sh", "scripts/railway/start-chain.sh"]
