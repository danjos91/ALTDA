FROM eclipse-temurin:21-jdk-jammy AS build
WORKDIR /app
COPY pom.xml .
COPY src ./src
RUN apt-get update && apt-get install -y -qq maven \
    && mvn -q -DskipTests package \
    && rm -rf /var/lib/apt/lists/*

FROM eclipse-temurin:21-jre-jammy
WORKDIR /app
COPY --from=build /app/target/java-service-*.jar /app/app.jar
EXPOSE 8000
ENV JAVA_OPTS=""
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar /app/app.jar"]
