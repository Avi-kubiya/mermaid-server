DOCKER_IMAGE=avi:latest
CONTAINER_NAME=avis

docker-image:
	docker build --platform linux/amd64 -t ${DOCKER_IMAGE} .

docker-run:
	docker run -d --name ${CONTAINER_NAME} -p 80:80 ${DOCKER_IMAGE}

docker-stop:
	docker stop ${CONTAINER_NAME} || true

docker-rm:
	make docker-stop
	docker rm ${CONTAINER_NAME} || true

docker-logs:
	docker logs -f ${CONTAINER_NAME}

docker-push:
	docker push ${DOCKER_IMAGE}
