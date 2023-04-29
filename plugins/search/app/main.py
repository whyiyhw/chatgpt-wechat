from fastapi import FastAPI
from langchain.tools import DuckDuckGoSearchRun
from langchain.utilities import WikipediaAPIWrapper
from pydantic import BaseModel

app = FastAPI()


class SearchQuery(BaseModel):
    command: str


class SearchResponse(BaseModel):
    msg: str


@app.post("/search", response_model=SearchResponse)
async def root(item: SearchQuery):
    tools = DuckDuckGoSearchRun()
    print("收到 info %s" % item.command)
    res = tools.run(item.command)
    print("返回 info %s" % res)
    return {"msg": res}


@app.post("/wikipedia", response_model=SearchResponse)
async def root(item: SearchQuery):
    tools = WikipediaAPIWrapper()
    print("收到 info %s" % item.command)
    res = tools.run(item.command)
    print("返回 info %s" % res)
    return {"msg": res}
