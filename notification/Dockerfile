FROM python:3.11-slim

WORKDIR /app

COPY notification/requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY notification/ .

CMD ["python", "bot.py"] 