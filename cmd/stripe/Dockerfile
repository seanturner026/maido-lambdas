FROM public.ecr.aws/lambda/provided:al2 as build

RUN yum install -y golang

RUN go env -w GOPROXY=direct

ADD go.mod go.sum ./

ADD . .

RUN go build -o /main ./cmd/stripe/*.go

FROM public.ecr.aws/lambda/provided:al2

COPY --from=build /main /main

ENTRYPOINT [ "/main" ]
