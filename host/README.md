# Delopy
1. Create dir /data/logs
2. Copy web.json and service.json to /data/logs
3. Change web.json and service.json to your settings
4. Run with docker
``` bash
docker run --name logs -d --restart always --network host -v /data/logs/web.json:/app/web.json -v /data/logs/service.json:/app/service.json dreamvat/logs
```