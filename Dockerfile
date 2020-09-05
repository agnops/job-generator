FROM golang:1.13-alpine AS build_base

RUN apk add --no-cache git

WORKDIR /tmp/app

COPY app/ .

RUN go mod download

RUN go build -o ./out/job-generator .

RUN ls -alt ./out/job-generator


FROM alpine:3.9 
RUN apk add ca-certificates

COPY --from=build_base /tmp/app/out/job-generator /app/job-generator

EXPOSE 3000

CMD ["/app/job-generator"]