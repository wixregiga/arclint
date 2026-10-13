# Sources behind the parser's decisions

The parser follows the pinned Context Mapper grammars (`../grammar/`) and the Xtext and ANTLR behavior below.
Each source is cited by its pinned URL; none is copied into this repository, and no Java, ANTLR or Xtext
runtime or generator is invoked by the package or its tests.

- Context Mapper 6.12.0 uses Xtext 2.32.0 (its build.gradle at context-mapper-dsl commit 995f486a9cdecb6946257d5c7b6d98ada2978124).
- Xtext tag v2.32.0 resolves to commit 8951042fa8d75e4ae1a813f07dd10f0755e99cfb.
- Terminals.xtext: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext/src/org/eclipse/xtext/common/Terminals.xtext
- UnorderedGroupHelper.java: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext/src/org/eclipse/xtext/parser/antlr/UnorderedGroupHelper.java
- STRINGValueConverter.java: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext/src/org/eclipse/xtext/conversion/impl/STRINGValueConverter.java
- AntlrGrammarGenerator.xtend: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.xtext.generator/src/org/eclipse/xtext/xtext/generator/parser/antlr/AntlrGrammarGenerator.xtend
- AbstractAntlrGrammarGenerator.xtend: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.xtext.generator/src/org/eclipse/xtext/xtext/generator/parser/antlr/AbstractAntlrGrammarGenerator.xtend
- AbstractAntlrGrammarWithActionsGenerator.xtend: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.xtext.generator/src/org/eclipse/xtext/xtext/generator/parser/antlr/AbstractAntlrGrammarWithActionsGenerator.xtend
- JavaStringConverter.java: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.util/src/org/eclipse/xtext/util/JavaStringConverter.java
- GrammarUtil.java: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext/src/org/eclipse/xtext/GrammarUtil.java
- KeywordHelper.java: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.xtext.generator/src/org/eclipse/xtext/xtext/generator/parser/antlr/KeywordHelper.java
- XtextAntlrGeneratorFragment2.xtend: https://github.com/eclipse/xtext/blob/8951042fa8d75e4ae1a813f07dd10f0755e99cfb/org.eclipse.xtext.xtext.generator/src/org/eclipse/xtext/xtext/generator/parser/antlr/XtextAntlrGeneratorFragment2.xtend
- AssignTokenTypesBehavior.java: https://github.com/antlr/antlr3/blob/3.5.3/tool/src/main/java/org/antlr/tool/AssignTokenTypesBehavior.java

Context Mapper 6.12.0 verdicts on generated and vendored inputs, recorded by an external run of its parser and
validators, are pinned in `context_mapper_6_12_verdicts.json.gz` (see `context_mapper_6_12_verdicts.md`).
