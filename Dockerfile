# build stage
ARG GO_VERS=1.18.2
FROM golang:${GO_VERS}-alpine AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tcn .

# final stage
FROM scratch
WORKDIR /app
COPY --from=build-env /build/tcn /app/

CMD ["/app/tcn"]
