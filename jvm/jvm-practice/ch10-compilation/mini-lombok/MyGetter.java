import java.lang.annotation.*;

// 클래스에 붙이면, 그 클래스의 모든 필드에 대한 getter 를 생성한다
@Target(ElementType.TYPE)
@Retention(RetentionPolicy.SOURCE)   // 컴파일 후엔 필요 없으니 SOURCE
public @interface MyGetter { }
