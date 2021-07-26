if [ -z "$1" ]; then
  echo "Provide log file please, like : ./waiting_launch.sh log.sh"
  exit
fi

PREV_MD5=""
MD5=""

while true
do
  sleep 10s
  MD5=$(md5sum $1 | cut -d ' ' -f1)
  if [ "$PREV_MD5" = "$MD5" ]; then
    exit
  fi
  echo "waiting... log file md5: $MD5"
  PREV_MD5=$MD5
done
