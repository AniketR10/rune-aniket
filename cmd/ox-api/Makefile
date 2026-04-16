ROOT_DIR ?= $(abspath ../..)
VERSION=$(shell git describe --tags)

.PHONY: docker-build-gcp docker-push-gcp

docker-build-gcp:
	@cd $(ROOT_DIR) && docker buildx build -f deploy/ox-api/Dockerfile --platform linux/amd64 -t us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/ox-api:$(VERSION) --build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" .

docker-push-gcp:
	@docker push us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/ox-api:$(VERSION)
