import os
from dotenv import load_dotenv

load_dotenv()

BOT_TOKEN = os.getenv('BOT_TOKEN')

WEBHOOK_HOST = os.getenv('WEBHOOK_HOST', 'https://your-domain.com')
WEBHOOK_PATH = os.getenv('WEBHOOK_PATH', '/webhook')  # Webhook path
WEBHOOK_PORT = int(os.getenv('WEBHOOK_PORT', '8443'))  # Webhook port

RABBITMQ_HOST = os.getenv('RABBITMQ_HOST', 'rabbitmq')
RABBITMQ_PORT = int(os.getenv('RABBITMQ_PORT', '5672'))
RABBITMQ_USER = os.getenv('RABBITMQ_USER', 'devuser')
RABBITMQ_PASSWORD = os.getenv('RABBITMQ_PASSWORD', 'admin')
RABBITMQ_QUEUE = os.getenv('RABBITMQ_QUEUE', 'telegram_notifications') 