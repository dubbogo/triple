package org.apache.dubbo;

import org.apache.dubbo.common.constants.CommonConstants;
import org.apache.dubbo.config.ApplicationConfig;
import org.apache.dubbo.config.ProtocolConfig;
import org.apache.dubbo.config.RegistryConfig;
import org.apache.dubbo.config.ServiceConfig;
import org.apache.dubbo.config.bootstrap.DubboBootstrap;

public class Provider {

    public static void main(String[] args) {
        ServiceConfig<IGreeter> service = new ServiceConfig<>();
        service.setInterface(IGreeter.class);
        service.setRef(new GreeterImpl());

        DubboBootstrap bootstrap = DubboBootstrap.getInstance();
       bootstrap.application(new ApplicationConfig("demo-provider"))
               .registry(new RegistryConfig("zookeeper://127.0.0.1:2181"))
               .protocol(new ProtocolConfig(CommonConstants.TRIPLE, 50051))
               .service(service)
               .start()
               .await();
    }

}
