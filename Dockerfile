FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /s2tbot .

# distroless/static ships the CA certificates needed for outgoing HTTPS.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /s2tbot /s2tbot

EXPOSE 8080
ENTRYPOINT ["/s2tbot"]
