import json
import asyncio
import logging
import aio_pika
from config import (
    RABBITMQ_HOST,
    RABBITMQ_PORT,
    RABBITMQ_USER,
    RABBITMQ_PASSWORD,
    RABBITMQ_QUEUE
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class RabbitMQConsumer:
    def __init__(self, bot):
        self.bot = bot
        self.connection = None
        self.channel = None
        self.queue = None
        self.connection_url = f"amqp://{RABBITMQ_USER}:{RABBITMQ_PASSWORD}@{RABBITMQ_HOST}:{RABBITMQ_PORT}/"

    async def connect(self):
        while True:
            try:
                logger.info(f"Attempting to connect to RabbitMQ at {RABBITMQ_HOST}:{RABBITMQ_PORT}")
                self.connection = await aio_pika.connect_robust(self.connection_url)
                self.channel = await self.connection.channel()
                self.queue = await self.channel.declare_queue(
                    RABBITMQ_QUEUE,
                    durable=True
                )
                logger.info("Successfully connected to RabbitMQ")
                return True
            except Exception as e:
                logger.error(f"Failed to connect to RabbitMQ: {str(e)}")
                logger.info("Retrying in 5 seconds...")
                await asyncio.sleep(5)

    async def process_message(self, message: aio_pika.IncomingMessage):
        async with message.process():
            try:
                data = json.loads(message.body.decode())
                telegram_id = data.get('telegram_id')
                status = data.get('status')
                company_name = data.get('company_name')
                job_title = data.get('job_title')

                logger.info(f"Received message: {data}")

                if not telegram_id or not status:
                    logger.warning(f"Invalid message format: {data}")
                    return

                try:
                    telegram_id = int(telegram_id)
                    logger.info(f"Converted telegram_id to integer: {telegram_id}")
                except (TypeError, ValueError):
                    logger.error(f"Invalid telegram_id format: {telegram_id}")
                    return
                status_translation = {
                    'Offered': 'Предложение',
                    'Interviewing': 'Собеседование',
                    'Rejected': 'Отказ',
                }
                message_text = f'Статус вашего отклика на работу (*{job_title}*) от *{company_name}* был изменен на "*{status_translation[status]}*"'
                logger.info(f"Preparing to send message: {message_text}")

                try:
                    logger.info(f"Attempting to send message to {telegram_id}")
                    await self.bot.send_message(
                        telegram_id, 
                        message_text,
                        parse_mode="Markdown"
                    )
                    logger.info(f"Message successfully sent to {telegram_id}")
                except Exception as e:
                    logger.error(f"Failed to send to {telegram_id}: {str(e)}")
                    raise

            except json.JSONDecodeError:
                logger.error("Invalid JSON format")
            except Exception as e:
                logger.error(f"Error processing message: {str(e)}")
                raise

    async def start_consuming(self):
        while True:
            try:
                if not self.connection or self.connection.is_closed:
                    await self.connect()

                logger.info("Starting to consume messages...")
                await self.queue.consume(self.process_message)
                
                # Keep the consumer running
                await asyncio.Future()
            except Exception as e:
                logger.error(f"Error in consumer: {str(e)}")
                if self.connection and not self.connection.is_closed:
                    await self.connection.close()
                logger.info("Retrying in 5 seconds...")
                await asyncio.sleep(5)

    async def close(self):
        if self.connection and not self.connection.is_closed:
            await self.connection.close() 