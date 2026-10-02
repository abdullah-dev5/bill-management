# Build a small static binary, then run it on a distroless image (~25 MB).
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
ENV APP_ENV=production PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
CMD ["serve"]
