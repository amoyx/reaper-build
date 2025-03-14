set -ex
curl -fsSL http://127.0.0.1/jdk-8u151-linux-x64.tar.gz -o  /tmp/jdk-8u151-linux-x64.tar.gz
mkdir -p /usr/local/java
tar -C  /usr/local/java  -xzf /tmp/jdk-8u151-linux-x64.tar.gz
