# api-pack

## 更新方法

开启go live (listen on 5500)
在wsl下

```bash
python gen-new.py 
```

api-pack目录下

```bash
ps -ef | grep api-pack
kill 

# copy this to command line, target server
DIR=/path/to/dir
DIR=/var/www/moonchan # *_secret_no_comment_*
cd $DIR/api-pack/   
pkill api-pack
sleep 1
curl localhost:5500/api-pack > api-pack
nohup ./api-pack > /dev/null 2>&1 &
```

## kv

GET
POST
DELETE

RESTful 键值对

## infonode

原来的helper server。
peers.


## reflect

返回http的头

