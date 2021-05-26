FROM ubuntu AS app
RUN apt-get update && apt-get install -y cowsay && rm -rf /var/lib/apt/lists/*

FROM redpwn/jail
COPY --from=app / /srv
COPY cowsay.sh /srv/app/run
