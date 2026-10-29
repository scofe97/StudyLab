FROM alpine:3.20

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV IMAGE_ONLY=image-default

ENTRYPOINT ["/entrypoint.sh"]
CMD ["8080"]
