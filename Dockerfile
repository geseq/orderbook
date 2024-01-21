FROM golang:1.19-alpine as go-base
RUN mkdir /app
COPY . /app
WORKDIR /app
RUN CGO_ENABLED=0 go build -o=bench /app/cmd/bench/main.go 

FROM scratch 
COPY --from=go-base /app/bench /bench
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/bench"]

CMD ["-duration=30", "-start-after=60"]
