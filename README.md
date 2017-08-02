总体介绍：
1、重载zip的readAt，用于读取数据，tcp读取客户端数据流，参数用json，主体逻辑在zip.go
2、实际zip的数据用文件缓存，在file.go中实现
3、对于音视频等资源文件，采用range参数返回，由于zip数据不支持seek，只能将其解压并缓存文件，在media.go中实现

主体流程：
zip的reader请求zip的基本信息-->tcp返回需求信息-->建立zip所有文件的服务--->根据请求文件名返回数据--->加载html时会请求很多资源--->采用多个zipparse去响应请求
