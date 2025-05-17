# Telegram Notification Bot

This bot reads messages from RabbitMQ and forwards them to subscribed Telegram users. It supports different types of messages (text, photos, documents) and handles them accordingly.

## Setup

1. Install the required dependencies:
```bash
pip install -r requirements.txt
```

2. Create a `.env` file with the following variables:
```
BOT_TOKEN=your_telegram_bot_token
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_QUEUE=telegram_notifications
```

3. Make sure RabbitMQ is running and accessible.

## Running the Bot

```bash
python bot.py
```

## Usage

1. Start a chat with the bot on Telegram
2. Send `/start` to subscribe to notifications
3. Send `/stop` to unsubscribe from notifications

## Message Format

The bot expects messages from RabbitMQ in the following JSON format:
```json
{
    "chat_id": 123456789,
    "message": "Your message here",
    "handler_type": "text"  // Optional: "text", "photo", or "document"
}
```

## Features

- Supports multiple message types (text, photos, documents)
- Handles connection errors gracefully
- Maintains a list of subscribed users
- Easy to extend with new message types 