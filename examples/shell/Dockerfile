FROM pwn.red/jail
COPY --from=ubuntu / /srv
RUN mkdir /srv/app && ln -s /bin/sh /srv/app/run
