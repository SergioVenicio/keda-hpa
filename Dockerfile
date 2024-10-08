FROM golang:1.23.2-bookworm AS BUILDER

WORKDIR /app
COPY ./app.go .
COPY ./go.mod .

RUN go build -o app .

FROM scratch

COPY --from=BUILDER /app/app . 

EXPOSE 5000
ENTRYPOINT [ "./app" ]