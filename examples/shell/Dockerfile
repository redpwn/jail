FROM redpwn/jail
COPY --from=ubuntu / /srv
RUN mkdir /srv/app && ln -s /bin/sh /srv/app/run
ENV JAIL_POW=5000
