# 1、从官方 Python 基础镜像开始
FROM python:3.11

# 导入环境变量 http_proxy
ARG http_proxy

ENV HTTP_PROXY=$http_proxy
ENV http_proxy=$http_proxy
ENV HTTPS_PROXY=$http_proxy
ENV https_proxy=$http_proxy

RUN apt-get update -y && apt-get install iputils-ping -y

# 2、将当前工作目录设置为 /code
# 这是放置 requirements.txt 文件和应用程序目录的地方
WORKDIR /code

# 3、先复制 requirements.txt 文件
# 由于这个文件不经常更改，Docker 会检测它并在这一步使用缓存，也为下一步启用缓存
COPY ./requirements.txt /code/requirements.txt

# 4、运行 pip 命令安装依赖项
RUN ping -c 1 -W 1 google.com > /dev/null \
    &&  pip install --no-cache-dir --upgrade -r /code/requirements.txt \
    || pip install --no-cache-dir --upgrade -r /code/requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

# 5、复制 FastAPI 项目代码
COPY ./app /code/app

# 6、运行服务
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8885"]