#!/usr/bin/env bash

header1="Content-type: application/json"
url="http://0.0.0.0:5988/chain/invoke"

postdata='
{
  "Color":"red",
  "ID":"car1",
  "Price":"10010",
  "LaunchDate":"11.66"
}
'
#echo $postdata
postdata=`echo $postdata | sed "s/ //g"`
#postdata=`echo $postdata | sed "s/" "/''/g"`
echo $postdata

curl -l -H "$header" -X POST -d $postdata $url
