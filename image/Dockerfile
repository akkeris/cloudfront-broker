FROM busybox

ARG NAME=cloudfront-broker

WORKDIR /opt/${NAME}
ADD ${NAME} .
CMD ./${NAME} --help
