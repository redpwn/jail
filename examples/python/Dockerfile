FROM python:slim AS app
RUN pip install --no-cache-dir progress

FROM pwn.red/jail
COPY --from=app / /srv
COPY multiply.py /srv/app/run
ENV JAIL_MEM=20M JAIL_POW=5000
