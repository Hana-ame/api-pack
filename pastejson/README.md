proxy的部分没啥问题,
总之做一个队列

post, get, put,
POST

```json
{
    "@metaData":{
        "tags":["{tags}"],
        "previous":["{id}"] ,
    }
    ...
}
```

返回:

```json
{
    "@metaData":{
        "tags":["{tags}"],
        "previous":["{id}"],
        "following":["{id}"],
    }
    ...
}
```

表

id catagory payload

tag id

id previous

实际上会存储原始内容但是会稍微读一下tags和previous

难不成都是一个小东小西然后加key加关系...
