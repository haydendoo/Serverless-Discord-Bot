#!/bin/bash
aws ecr get-login-password --region ap-southeast-1 | docker login --username AWS --password-stdin 314146331900.dkr.ecr.ap-southeast-1.amazonaws.com
docker build -t upload-bot .
docker tag upload-bot:latest 314146331900.dkr.ecr.ap-southeast-1.amazonaws.com/upload-bot:latest
docker push 314146331900.dkr.ecr.ap-southeast-1.amazonaws.com/upload-bot:latest