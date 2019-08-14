#!/usr/bin/env bash

header1="Content-type: application/json"
url="http://0.0.0.0:5988/chain/queryRange"

postdata='
{
  "Args":["car1","car3"]
}
'
#echo $postdata
postdata=`echo $postdata | sed "s/ //g"`
#postdata=`echo $postdata | sed "s/" "/''/g"`
echo $postdata

curl -l -H "$header" -X POST -d $postdata $url
