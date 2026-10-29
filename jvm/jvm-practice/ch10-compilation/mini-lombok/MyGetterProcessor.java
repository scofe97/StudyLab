import javax.annotation.processing.*;
import javax.lang.model.element.*;
import javax.lang.model.SourceVersion;
import javax.tools.JavaFileObject;
import java.io.Writer;
import java.util.Set;

@SupportedAnnotationTypes("MyGetter")
@SupportedSourceVersion(SourceVersion.RELEASE_21)
public class MyGetterProcessor extends AbstractProcessor {

    @Override
    public boolean process(Set<? extends TypeElement> annotations, RoundEnvironment roundEnv) {
        // @MyGetter 가 붙은 모든 클래스에 대해
        for (Element annotated : roundEnv.getElementsAnnotatedWith(
                processingEnv.getElementUtils().getTypeElement("MyGetter"))) {
            TypeElement clazz = (TypeElement) annotated;
            generateGetterClass(clazz);
        }
        return true;   // 이 애너테이션은 우리가 처리 완료
    }

    private void generateGetterClass(TypeElement clazz) {
        String origName = clazz.getSimpleName().toString();   // 예: User
        String genName  = origName + "Getters";               // 예: UserGetters
        StringBuilder sb = new StringBuilder();
        sb.append("public final class ").append(genName).append(" {\n");

        // 클래스의 각 필드마다 getter 메서드를 문자열로 조립
        for (Element member : clazz.getEnclosedElements()) {
            if (member.getKind() == ElementKind.FIELD) {
                String field = member.getSimpleName().toString();          // name
                String type  = member.asType().toString();                 // java.lang.String
                String cap   = Character.toUpperCase(field.charAt(0)) + field.substring(1);
                sb.append("    public static ").append(type).append(" get").append(cap)
                  .append("(").append(origName).append(" o) { return o.").append(field).append("; }\n");
            }
        }
        sb.append("}\n");

        // 새 소스 파일을 실제로 "생성" — 라운드가 반복되며 이 코드도 컴파일된다
        try {
            JavaFileObject f = processingEnv.getFiler().createSourceFile(genName);
            try (Writer w = f.openWriter()) { w.write(sb.toString()); }
            processingEnv.getMessager().printMessage(
                javax.tools.Diagnostic.Kind.NOTE, "생성됨: " + genName + ".java");
        } catch (Exception ex) {
            processingEnv.getMessager().printMessage(
                javax.tools.Diagnostic.Kind.ERROR, "생성 실패: " + ex);
        }
    }
}
