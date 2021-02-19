FROM python:slim AS app

RUN pip install progress --no-cache-dir

FROM redpwn/jail

COPY --from=app / /app
COPY multiply.py /app/app/challenge

ENV JAIL_MEM 20971520
