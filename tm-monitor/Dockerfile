FROM alpine:3.5

WORKDIR /app
COPY tm-monitor /app/tm-monitor

# look for local Tendermint node by default
CMD ["localhost:46657"]

ENTRYPOINT ["./tm-monitor"]
