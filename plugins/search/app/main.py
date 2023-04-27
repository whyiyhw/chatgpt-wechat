import os
from fastapi import FastAPI
from langchain.tools import DuckDuckGoSearchRun
from pydantic import BaseModel

os.environ["HTTP_PROXY"] = "http://192.168.1.202:1081"
os.environ["http_proxy"] = "http://192.168.1.202:1081"
os.environ["HTTPS_PROXY"] = "http://192.168.1.202:1081"
os.environ["https_proxy"] = "http://192.168.1.202:1081"

app = FastAPI()


class SearchQuery(BaseModel):
    command: str


class SearchResponse(BaseModel):
    msg: str


@app.post("/chat", response_model=SearchResponse)
async def root(item: SearchQuery):
    tools = DuckDuckGoSearchRun()
    print("收到 info %s" % item.command)
    res = tools.run(item.command)
    print("返回 info %s" % res)
    return {"msg": res}
