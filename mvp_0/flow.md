# flow

curl -X POST "https://mycompanycurl -X POST "https://mycompanygroup.demo.saas.com/api/refreshtoken" \
 -H "Content-Type: application/json" \
 -d '{
"user": {
"LoginName": "saas_Integration",
"Loginpassword": "saas123$"
}
}'
{"Token":"becb2bbeb2344f8e80eee4e0f9082283dab0d3f9aee54642a349d0dca339c266:1QAAAA==","RefreshDateTime":"2026-04-20T15:22:41","ExpiryDateTime":"2026-04-21T15:22:41"}

curl -X GET "https://mycompanygroup.demo.saas.com/api/token/" -H "Authorization: Bearer e03405c8ba36409eab2b658131688d74c7b0f0e6^C
zb_bamboo@blackdesk:~/DEV/**NEW**/Python/saas_uploader$ curl -X GET "https://mycompanygroup.demo.saas.com/api/token/" -H "Authorization: Bearer becb2bbeb2344f8e80eee4e0f9082283dab0d3f9aee54642a349d0dca339c266:1QAAAA=="
{"AccessToken":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwOi8vc2NoZW1hcy5taWNyb3NvZnQuY29tL3dzLzIwMDgvMDYvaWRlbnRpdHkvY2xhaW1zL3VzZXJkYXRhIjoiMzQiLCJ1bmlxdWVfbmFtZSI6ImJtd2dyb3VwLmRlbW8uY29yaXR5LmNvbSIsIm5iZiI6MTc3NjY5NTAyNywiZXhwIjoxNzc2Njk1MzI3LCJpYXQiOjE3NzY2OTUwMjd9.7Ivm7O7M4kB7dgMEVpw9rDDP9NEVikg0kuqbIp_D25k","RefreshToken":null,"AccessTokenExpiryDateTime":"2026-04-20T14:28:47"}
