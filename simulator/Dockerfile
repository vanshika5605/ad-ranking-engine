FROM python:3.12-slim
WORKDIR /app
COPY simulate.py .
# No deps: uses stdlib only
ENTRYPOINT ["python", "simulate.py"]
CMD ["--requests", "500", "--click-rate", "0.03"]
