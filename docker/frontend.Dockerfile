FROM node:24-alpine AS builder

ARG APP_DIR
ARG VITE_API_BASE_URL=/
ARG VITE_WS_BASE_URL=
ARG NPM_CONFIG_REGISTRY=https://registry.npmjs.org
WORKDIR /src
ENV NPM_CONFIG_REGISTRY=${NPM_CONFIG_REGISTRY}
RUN for attempt in 1 2 3 4 5; do \
      npm install --global pnpm@11.25.0 --registry="${NPM_CONFIG_REGISTRY}" && break; \
      test "$attempt" = 5 && exit 1; sleep 3; \
    done
COPY ${APP_DIR}/package.json ${APP_DIR}/pnpm-lock.yaml ${APP_DIR}/pnpm-workspace.yaml ./
RUN --mount=type=cache,id=gim-pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm config set registry "${NPM_CONFIG_REGISTRY}" && \
    for attempt in 1 2 3 4 5; do \
      pnpm install --frozen-lockfile --network-concurrency=1 --fetch-timeout=600000 --fetch-retries=8 && break; \
      test "$attempt" = 5 && exit 1; sleep 3; \
    done
COPY ${APP_DIR}/ ./
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_WS_BASE_URL=${VITE_WS_BASE_URL}
RUN pnpm build

FROM nginxinc/nginx-unprivileged:1.29-alpine
COPY docker/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist/ /usr/share/nginx/html/
EXPOSE 8080
