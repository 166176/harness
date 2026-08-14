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
# 大陆网络镜像：默认 proxy.golang.org 不可达（冷启动验证已证）
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# vite outDir 为 ../internal/server/webui/dist（go:embed 落点），覆盖仓库内提交的旧产物
COPY --from=webui /app/internal/server/webui/dist ./internal/server/webui/dist
RUN CGO_ENABLED=0 go build -o /gavel ./cmd/gavel

# 阶段3 运行时（alpine 取自 Docker Hub，大陆可拉；distroless 的 gcr.io 常被墙）
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /gavel /gavel
EXPOSE 8080
ENTRYPOINT ["/gavel", "serve"]
