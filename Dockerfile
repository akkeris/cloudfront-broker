FROM alpine:3.8

ARG NAME=cloudfront-broker
WORKDIR /app
ENV NAME ${NAME}
COPY  ${NAME} /app/${NAME}
CMD /app/${NAME}  -logtostderr=1 -stderrthreshold 0


