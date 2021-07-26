package org.apache.dubbo;

import org.apache.dubbo.common.constants.CommonConstants;
import org.apache.dubbo.config.ApplicationConfig;
import org.apache.dubbo.config.ReferenceConfig;
import org.apache.dubbo.config.RegistryConfig;
import org.apache.dubbo.config.bootstrap.DubboBootstrap;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

public class Consumer {

    public static void main(String[] args) throws IOException {
        ReferenceConfig<IGreeter> ref = new ReferenceConfig<>();
        ref.setInterface(IGreeter.class);
        ref.setCheck(false);
        ref.setProtocol(CommonConstants.TRIPLE);
        ref.setLazy(true);
        ref.setTimeout(100000);

        DubboBootstrap bootstrap = DubboBootstrap.getInstance();
       bootstrap.application(new ApplicationConfig("demo-consumer"))
               .registry(new RegistryConfig("zookeeper://127.0.0.1:2181"))
               .reference(ref)
               .start();

        final IGreeter iGreeter = ref.get();
        try {
            final User user = iGreeter.sayHello(HelloRequest.newBuilder()
                    .setName("xavier")
                    .build());
            TimeUnit.SECONDS.sleep(1);
            System.out.println("User name: " + user.getName() + ", age: " + user.getAge() + ", id: " + user.getId());
        } catch (Throwable t) {
            t.printStackTrace();
        }
        System.in.read();
    }

}
