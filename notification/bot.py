import asyncio
import logging
import ssl
from aiohttp import web
from aiogram import Bot, Dispatcher, types
from aiogram.filters.command import Command
from aiogram.webhook.aiohttp_server import SimpleRequestHandler, setup_application
from config import BOT_TOKEN, WEBHOOK_HOST, WEBHOOK_PATH, WEBHOOK_PORT
from rabbitmq_consumer import RabbitMQConsumer

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

logger.info("Initializing bot with token...")
bot = Bot(token=BOT_TOKEN)
dp = Dispatcher()

logger.info("Initializing RabbitMQ consumer...")
consumer = RabbitMQConsumer(bot)
logger.info("RabbitMQ consumer initialized")

@dp.message(Command("start"))
async def cmd_start(message: types.Message):
    logger.info(f"Received /start command from user {message.from_user.id}")
    await message.answer("Привет! Я бот для уведомление сервиса НайкPro")

async def on_startup(application: web.Application) -> None:
    webhook_url = f"{WEBHOOK_HOST}{WEBHOOK_PATH}"
    logger.info(f"Setting webhook to {webhook_url}")
    await bot.delete_webhook(drop_pending_updates=True)

    await bot.set_webhook(
        url=webhook_url,
        drop_pending_updates=True,
        allowed_updates=["message", "callback_query"]
    )

    webhook_info = await bot.get_webhook_info()
    logger.info(f"Webhook info: {webhook_info}")
    
    if webhook_info.url != webhook_url:
        logger.error(f"Webhook URL mismatch! Expected: {webhook_url}, Got: {webhook_info.url}")
    else:
        logger.info("Webhook set successfully")

async def on_shutdown(application: web.Application) -> None:
    logger.info("Removing webhook...")
    await bot.delete_webhook()
    logger.info("Webhook removed")

async def run_consumer():
    try:
        logger.info("Starting RabbitMQ consumer...")
        await consumer.start_consuming()
    except Exception as e:
        logger.error(f"Error in consumer: {str(e)}")
        raise

async def main():
    try:
        logger.info("Starting main application...")

        app = web.Application()

        webhook_handler = SimpleRequestHandler(
            dispatcher=dp,
            bot=bot,
        )
        webhook_handler.register(app, path=WEBHOOK_PATH)

        setup_application(app, dp, bot=bot)

        app.on_startup.append(on_startup)
        app.on_shutdown.append(on_shutdown)

        consumer_task = asyncio.create_task(run_consumer())
        logger.info("Consumer task created")
        

        logger.info(f"Starting web server on port {WEBHOOK_PORT}...")
        await web._run_app(
            app,
            host="0.0.0.0",
            port=WEBHOOK_PORT,
        )
        
    except Exception as e:
        logger.error(f"Error in main: {str(e)}")
        raise
    finally:
        logger.info("Cleaning up resources...")
        if consumer.connection and not consumer.connection.is_closed:
            await consumer.connection.close()
        await bot.session.close()
        logger.info("Cleanup completed")

if __name__ == "__main__":
    try:
        logger.info("Starting application...")
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Bot stopped by user")
    except Exception as e:
        logger.error(f"Bot stopped due to error: {str(e)}") 