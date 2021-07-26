package org.apache.dubbo;

public class GreeterImpl implements IGreeter {
    @Override
    public User sayHello(HelloRequest request) {
        return User.newBuilder()
                .setName(request.getName())
                .setId("12345")
                .setAge(21)
                .build();
    }
}
