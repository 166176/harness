# 阶段1 前端
FROM node:22-alpine AS webui
WORKDIR /app/webui
COPY webui/package*.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build

# 阶段2 Go（go.mod 要求 go 1.24+：测试/代码使用 t.Context）
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# vite outDir 为 ../internal/server/webui/dist（go:embed 落点），覆盖仓库内提交的旧产物
COPY --from=webui /app/internal/server/webui/dist ./internal/server/webui/dist
RUN CGO_ENABLED=0 go build -o /gavel ./cmd/gavel

# 阶段3 运行时
FROM gcr.io/distroless/static-debian12
COPY --from=build /gavel /gavel
EXPOSE 8080
ENTRYPOINT ["/gavel", "serve"]
