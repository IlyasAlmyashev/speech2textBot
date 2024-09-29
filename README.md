Speech2text telegram bot for voice messages

to run pass environmental variables:

BOT_TOKEN - get here https://t.me/BotFather
BOT_URL - get from deployment provider or ngrok if you run locally
ASSEMBLY_KEY - get here https://www.assemblyai.com

for example:

docker run -d 
-e BOT_TOKEN='xxx' 
-e BOT_URL='xxx' 
-e ASSEMBLY_KEY='xxx' 
-p 8080:8080 s2tbot