#
#  Licensed to the Apache Software Foundation (ASF) under one or more
#  contributor license agreements.  See the NOTICE file distributed with
#  this work for additional information regarding copyright ownership.
#  The ASF licenses this file to You under the Apache License, Version 2.0
#  (the "License"); you may not use this file except in compliance with
#  the License.  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

if [ -z "$1" ]; then
  echo "Provide test directory please, like : ./integrate_test.sh helloworld/go-server"
  exit
fi

P_DIR=$(pwd)/$1

make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile docker-up

# check docker health
make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile docker-health-check

# start server
make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile start
# start integration
make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile integration
result=$?

# if fail print server log
if [ $result != 0 ];then
  make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile print-server-log
fi

# stop server
make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile clean

# test java server and go client
# if you want to this test, please make sure`java-server` where contains `run.sh` existed
JAVA_SERVER_SHELL=$(pwd)/$(dirname $1)/java-server
if [ -e $JAVA_SERVER_SHELL ]; then
  # start java server
  make PROJECT_DIR=$JAVA_SERVER_SHELL PROJECT_NAME=$(basename $JAVA_SERVER_SHELL) BASE_DIR=$JAVA_SERVER_SHELL/dist -f build/Makefile start-java

  # start integration
  make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile build
  make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile integration
  result=$?

  # if fail print server log
  if [ $result != 0 ];then
    make PROJECT_DIR=$JAVA_SERVER_SHELL PROJECT_NAME=$(basename $JAVA_SERVER_SHELL) BASE_DIR=$JAVA_SERVER_SHELL/dist -f build/Makefile print-server-log
  fi

  # stop server
  make PROJECT_DIR=$JAVA_SERVER_SHELL PROJECT_NAME=$(basename $JAVA_SERVER_SHELL) BASE_DIR=$JAVA_SERVER_SHELL/dist -f build/Makefile clean
fi

make PROJECT_DIR=$P_DIR PROJECT_NAME=$(basename $P_DIR) BASE_DIR=$P_DIR/dist -f build/Makefile docker-down

exit $((result))
