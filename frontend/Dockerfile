FROM node:22-alpine AS builder
ARG BUILD_COMMIT=dev

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend ./
RUN npm run build

FROM nginx:1.27-alpine
ARG BUILD_COMMIT=dev

COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/frontend/dist /usr/share/nginx/html
RUN printf '{"commit":"%s"}\n' "$BUILD_COMMIT" > /usr/share/nginx/html/build.json

EXPOSE 8080
