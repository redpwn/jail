FROM python:slim AS app
RUN pip install progress --no-cache-dir

FROM redpwn/jail
COPY --from=app / /srv
COPY multiply.py /srv/app/run
ENV JAIL_MEM=20M JAIL_POW=5000
