# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26.0 as build

# git and openssh needed to go get private repos
RUN apt-get update && apt-get install -y --no-install-recommends git openssh-client ca-certificates && rm -rf /var/lib/apt/lists/*

ARG GIT_SSH_KEY
ARG GCP_PROJECT="unstable-build-blue-dev"
ARG TARGETOS
ARG TARGETARCH
# OXAPI_LDFLAGS is appended to `go build -ldflags` and is used to inject
# the production auth0 / signup endpoints into the binary. Empty by
# default (dev endpoints baked in by source).
ARG OXAPI_LDFLAGS=""

WORKDIR /src

COPY . .

RUN if [ -n "$GIT_SSH_KEY" ]; then git config --global url.ssh://git@github.com/.insteadOf https://github.com/; fi
RUN mkdir -p ~/.ssh && chmod 700 ~/.ssh && \
	if [ -n "$GIT_SSH_KEY" ]; then \
		printf '%s\n' "$GIT_SSH_KEY" | tr -d '\r' > ~/.ssh/id_rsa && \
		chmod 600 ~/.ssh/id_rsa; \
	fi
RUN ssh-keyscan -t rsa git.unstable.build >> ~/.ssh/known_hosts
RUN ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts
RUN go env -w GOPRIVATE="github.com/unstablebuild,unstable.build/*"
RUN cd cmd/ox-api && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="$OXAPI_LDFLAGS" -o /bin/ox-api .

# use clean image with no deps or install
FROM --platform=linux/amd64 alpine:latest
ARG GCP_PROJECT="unstable-build-blue-dev"
ENV GCP_PROJECT=$GCP_PROJECT
COPY --from=build /bin/ox-api /bin/ox-api

EXPOSE 4433
EXPOSE 3001

CMD [ "sh", "-c", "/bin/ox-api -G 4433 -P 3001 -p ${GCP_PROJECT} -H 0.0.0.0 -I -J" ]
