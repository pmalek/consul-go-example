FROM golang:1.14-alpine AS builder

WORKDIR /go/src
COPY go.mod .
RUN go mod download
COPY ./ /go/src
RUN cd backend && go build -v -o /go/backend .

FROM alpine:3.11

# WORKDIR /go
COPY --from=builder /go/backend /bin
CMD ["backend"]