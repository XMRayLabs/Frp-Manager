# 閰嶇疆璇存槑

## frp闅ч亾楂樼骇妯″紡閰嶇疆

鏈潰鏉垮畬鍏ㄥ吋瀹?frp 鍘熸湰鐨刞json`鏍煎紡閰嶇疆锛屼粎闇€瑕佸皢閰嶇疆鏂囦欢鍐呭绮樿创鍒版湇鍔＄/瀹㈡埛绔珮绾фā寮忕紪杈戞鍐咃紝鏇存柊鍗冲彲锛岃缁嗙殑浣跨敤鍙傝€冿細[frp 鏂囨。](https://gofrp.org/zh-cn/docs/features/common/configure/)

## 绋嬪簭鍚姩閰嶇疆鏂囦欢

绋嬪簭浼氭寜椤哄簭璇诲彇浠ヤ笅鏂囦欢鍐呭浣滀负閰嶇疆鏂囦欢锛歚.env`,`/etc/frpp/.env`

## 绋嬪簭閰嶇疆璇存槑

> 鏂囨。鍙兘鏈夌偣鑰併€傘€傘€?
> 
> 瀹屾暣鐨勬渶鏂伴厤缃弬鑰冭繖涓枃浠讹細[settings.go](https://github.com/Sakurame1/frp-manager/blob/main/conf/settings.go)

| 绫诲瀷   | 鐜鍙橀噺鍚?                            | 榛樿鍊?              | 鎻忚堪                                                             |
|--------|-------------------------------------|--------------------|----------------------------------------------------------------|
| string | `APP_SECRET`                       | -                  | 搴旂敤瀵嗛挜锛岀敤浜庡鎴风鍜屾湇鍔″櫒鐨勫拰Master鐨勯€氫俊鍔犲瘑                        |
| string | `APP_GLOBAL_SECRET`                | 空值时运行时生成    | 全局密钥；生产环境请设置长随机值，避免重启后会话失效。                      |
| int    | `APP_COOKIE_AGE`                   | `86400`            | Cookie 鐨勬湁鏁堟湡锛堢锛夛紝榛樿鍊间负 1 澶?                                 |
| string | `APP_COOKIE_NAME`                  | `frp-manager-cookie` | Cookie 鍚嶇О                                                        |
| string | `APP_COOKIE_PATH`                  | `/`                | Cookie 璺緞                                                       |
| string | `APP_COOKIE_DOMAIN`                | -                  | Cookie 鍩?                                                        |
| bool   | `APP_COOKIE_SECURE`                | `false`            | Cookie 鏄惁瀹夊叏                                                   |
| bool   | `APP_COOKIE_HTTP_ONLY`             | `true`             | Cookie 鏄惁浠呴檺 HTTP                                             |
| bool   | `APP_ENABLE_REGISTER`              | `false`            | 显式开启用户注册，受控创建账号后应立即关闭                                 |
| bool   | `APP_AUTO_UPDATE`                  | `true`             | Client/Server 每次启动时检查、校验并安装新版本                           |
| string | `LOGGER_FILE`                     | -                  | 可选日志文件路径；GUI 托管服务使用 `service.log`                         |
| int    | `MASTER_API_PORT`                  | `9000`             | 涓昏妭鐐?API 绔彛                                                  |
| string | `MASTER_API_HOST`                  | -                  | 涓昏妭鐐瑰煙鍚嶏紝鍙互鍦ㄥ弽鍚戜唬鐞嗗拰CDN鍚?                                |
| string | `MASTER_API_SCHEME`                | `http`             | 涓昏妭鐐?API 鍗忚锛堟敞鎰忥紝杩欓噷涓嶅奖鍝嶄富鏈鸿涓猴紝璁剧疆涓篽ttps鍙槸涓轰簡鏂逛究澶嶅埗瀹㈡埛绔惎鍔ㄥ懡浠わ紝HTTPS闇€瑕佽嚜琛屽弽鍚戜唬鐞嗭級|
| int    | `MASTER_CACHE_SIZE`                | `10`               | 缂撳瓨澶у皬锛圡B锛?                                                  |
| string | `MASTER_RPC_HOST`                  | `127.0.0.1`        | Master鑺傜偣鍏叡 IP 鎴栧煙鍚?                                         |
| int    | `MASTER_RPC_PORT`                  | `9001`             | Master鑺傜偣 RPC 绔彛                                            |
| bool   | `MASTER_COMPATIBLE_MODE`           | `false`            | 鍏煎妯″紡锛岀敤浜庡畼鏂?frp 瀹㈡埛绔?                                    |
| string | `MASTER_INTERNAL_FRP_SERVER_HOST`  | -                  | Master鍐呯疆 frps 鏈嶅姟鍣ㄤ富鏈猴紝鐢ㄤ簬瀹㈡埛绔繛鎺?                               |
| int    | `MASTER_INTERNAL_FRP_SERVER_PORT`  | `9002`             | Master鍐呯疆 frps 鏈嶅姟鍣ㄧ鍙ｏ紝鐢ㄤ簬瀹㈡埛绔繛鎺?                               |
| string | `MASTER_INTERNAL_FRP_AUTH_SERVER_HOST` | `127.0.0.1`    | Master鍐呯疆 frps 璁よ瘉鏈嶅姟鍣ㄤ富鏈?                                         |
| int    | `MASTER_INTERNAL_FRP_AUTH_SERVER_PORT` | `8999`          | Master鍐呯疆 frps 璁よ瘉鏈嶅姟鍣ㄧ鍙?                                         |
| string | `MASTER_INTERNAL_FRP_AUTH_SERVER_PATH` | `/auth`         | Master鍐呯疆 frps 璁よ瘉鏈嶅姟鍣ㄨ矾寰?                                         |
| int    | `SERVER_API_PORT`                  | `8999`             | 鏈嶅姟鍣?API 绔彛                                                  |
| string | `DB_TYPE`                          | `sqlite3`         | 鏁版嵁搴撶被鍨嬶紝濡?mysql postgres 鎴?sqlite3 绛?                                |
| string | `DB_DSN`                           | `data.db`         | 鏁版嵁搴?DSN锛岄粯璁や娇鐢╯qlite3锛屾暟鎹粯璁ゅ瓨鍌ㄥ湪鍙墽琛屾枃浠跺悓鐩綍涓嬶紝瀵逛簬 sqlite 鏄矾寰勶紝鍏朵粬鏁版嵁搴撲负 DSN锛屽弬瑙?[MySQL DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name) |
| string | `CLIENT_ID`                        | -                  | 瀹㈡埛绔?ID                                                        |
| string | `CLIENT_SECRET`                   | -                  | 瀹㈡埛绔瘑閽?                                                      |
| bool   | `IS_DEBUG`                         | `false`            | 鏄惁寮€鍚皟璇曟ā寮忥紙褰卞搷鏃ュ織/閮ㄥ垎缁勪欢琛屼负锛?                                 |
| bool   | `DEBUG_PROFILER_ENABLED`           | `false`            | 鏄惁寮€鍚?profiler(pprof) HTTP 鏈嶅姟锛堥粯璁や粎鐩戝惉 127.0.0.1锛?                |
| int    | `DEBUG_PROFILER_PORT`              | `6961`             | profiler(pprof) HTTP 鏈嶅姟绔彛                                      |
