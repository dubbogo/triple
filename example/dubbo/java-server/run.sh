if [ -z "$1" ]; then
  echo "Provide test directory please, like : ./run.sh $(pwd)/helloworld/java-server"
  exit
fi

cd $1

mvn install -DSkipTests
mvn exec:java -Dexec.mainClass="org.apache.dubbo.Provider"