FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN LDFLAGS="$(sh scripts/ldflags.sh)" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="$LDFLAGS" -o /out/trishna ./cmd/trishna

FROM alpine:3.22

RUN adduser -D -H -u 10001 trishna
USER trishna

COPY --from=build /out/trishna /usr/local/bin/trishna

ENTRYPOINT ["trishna"]
