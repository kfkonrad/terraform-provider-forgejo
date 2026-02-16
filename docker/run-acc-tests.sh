#/usr/bin/env bash

docker compose -f docker/docker-compose.yml down -v  &> /dev/null
docker compose -f docker/docker-compose.yml up --no-start  &> /dev/null
docker run --rm -v docker_forgejo:/data alpine mkdir -p /data/gitea/conf  &> /dev/null
docker compose -f docker/docker-compose.yml cp docker/app.ini forgejo:/data/gitea/conf/app.ini  &> /dev/null
docker compose -f docker/docker-compose.yml start  &> /dev/null

export FORGEJO_USERNAME=tfadmin
export FORGEJO_PASSWORD=$(openssl rand -base64 32)
while ! docker compose -f docker/docker-compose.yml exec -u git \
  forgejo /usr/local/bin/forgejo admin user create \
  --username $FORGEJO_USERNAME \
  --email $FORGEJO_USERNAME@localhost \
  --password "$FORGEJO_PASSWORD" \
  --must-change-password=false \
  --admin &> /dev/null
do
  sleep 2
done

export TF_ACC=1
if [[ -z $@ ]]; then
  go test -v -cover -timeout 120m ./internal/...
else
  go test -v -cover -timeout 120m ./internal/provider/provider_test.go $@
fi

docker compose -f docker/docker-compose.yml down -v  &> /dev/null
