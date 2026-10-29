import javax.annotation.processing.*;
import javax.lang.model.element.*;
import javax.lang.model.util.ElementScanner8;
import javax.lang.model.SourceVersion;
import javax.tools.Diagnostic;
import java.util.Set;

// 모든 애너테이션을 처리 대상으로 — 명명 규약은 애너테이션과 무관하게 전부 검사
@SupportedAnnotationTypes("*")
@SupportedSourceVersion(SourceVersion.RELEASE_17)
public class NameCheckProcessor extends AbstractProcessor {

    @Override
    public boolean process(Set<? extends TypeElement> annotations, RoundEnvironment roundEnv) {
        if (!roundEnv.processingOver()) {
            NameCheckScanner scanner = new NameCheckScanner();
            for (Element e : roundEnv.getRootElements()) {
                scanner.scan(e);
            }
        }
        return false;   // 다른 처리기에 양보
    }

    // ElementScanner(visitor)로 구문 트리 순회 — 종류별로 다른 visit 메서드가 호출됨
    private class NameCheckScanner extends ElementScanner8<Void, Void> {
        @Override
        public Void visitType(TypeElement e, Void p) {            // 클래스 → UpperCamelCase
            String n = e.getSimpleName().toString();
            if (!n.isEmpty() && !Character.isUpperCase(n.charAt(0))) {
                warn(e, "클래스 '" + n + "' 는 UpperCamelCase 가 아닙니다");
            }
            return super.visitType(e, p);
        }
        @Override
        public Void visitExecutable(ExecutableElement e, Void p) { // 메서드 → lowerCamelCase
            if (e.getKind() == ElementKind.METHOD) {
                String n = e.getSimpleName().toString();
                if (!n.isEmpty() && !Character.isLowerCase(n.charAt(0))) {
                    warn(e, "메서드 '" + n + "' 는 lowerCamelCase 가 아닙니다");
                }
            }
            return super.visitExecutable(e, p);
        }
        @Override
        public Void visitVariable(VariableElement e, Void p) {     // static final → ALL_CAPS
            Set<Modifier> mods = e.getModifiers();
            String n = e.getSimpleName().toString();
            if (mods.contains(Modifier.STATIC) && mods.contains(Modifier.FINAL)
                    && !n.equals(n.toUpperCase())) {
                warn(e, "상수 '" + n + "' 는 ALL_CAPS 가 아닙니다");
            }
            return super.visitVariable(e, p);
        }
    }

    private void warn(Element e, String msg) {
        processingEnv.getMessager().printMessage(Diagnostic.Kind.WARNING, msg, e);
    }
}
