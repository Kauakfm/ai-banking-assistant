# BFA — Back-end for Agents: Guia Completo do Padrão Arquitetural

> Documento baseado no artigo original de **Michael Douglas Barbosa Araujo** — Staff Architect AI @ Itaú.  
> Fonte: [O padrão Back-end para Agentes (BFA)](https://medium.com/@mdbaraujo/o-padr%C3%A3o-back-end-para-agentes-bfa-a53c1c6d87fb)

---

## 1. O que é o BFA?

O **BFA (Back-end for Agents)** é um design pattern arquitetural que introduz uma **camada intermediária** entre os agentes de IA e as APIs/serviços de domínio. Ele é inspirado diretamente no padrão **BFF (Back-end for Front-end)**, proposto por Phil Calçado no SoundCloud, mas aplicado ao contexto de agentes de inteligência artificial.

A ideia central é que o agente **não deve chamar diretamente** as APIs de domínio (crédito, saldo, cadastro, etc.). Em vez disso, ele invoca **operações padronizadas** expostas por um BFA, que é responsável por toda a lógica de backend necessária para o funcionamento do agente.

### Definição em uma frase

> O BFA é uma camada mediadora que encapsula APIs, aplica políticas, traduz dados e expõe operações estáveis via protocolos (como MCP) para os agentes de IA.

---

## 2. Por que o BFA existe? — O Problema

### Analogia com a evolução do software tradicional

Historicamente, ao criar um sistema nasciam um backend e um frontend acoplados. O grande problema sempre foi o **reaproveitamento**: duplicações de classes, métodos e regras idênticas espalhadas por diferentes partes do sistema. Phil Calçado identificou esse problema e propôs o BFF para resolvê-lo na camada de frontend.

O mesmo problema agora se repete na era dos agentes de IA. Os agentes tendem a se acoplar diretamente às APIs de domínio, gerando os seguintes problemas:

### Problemas da arquitetura sem BFA

| Problema | Descrição |
|---|---|
| **Acoplamento forte** | Lógica de APIs, tradução semântica e decisões misturadas. Qualquer alteração na API pode quebrar o agente. |
| **Reuso limitado** | Se outro agente precisar da mesma informação, é necessário reimplementar a lógica. |
| **Escalabilidade e evolução difíceis** | É difícil testar e fazer rollout de partes isoladas. |
| **Segurança dispersa** | Cada agente pode implementar seu próprio padrão de segurança — sem padronização. |
| **Observabilidade fragmentada** | Não há fronteira clara para métricas, traces e fallback. |

Esses são problemas clássicos de **aplicações monolíticas**, agora replicados no mundo dos agentes.

---

## 3. Como o BFA resolve — A Solução

O BFA atua como uma **camada mediadora** entre o agente (que contém o LLM, o prompt e a memória) e as APIs de domínio.

### Arquitetura do BFA

```
┌─────────────────────────────────────────────────────────┐
│                      CLIENTE                            │
│          "Posso usar meu limite disponível?"            │
└──────────────────────┬──────────────────────────────────┘
                       │
              ┌────────▼────────┐
              │   SUPERVISOR    │
              │   (LangGraph)   │
              └───┬─────────┬───┘
                  │         │
     ┌────────────▼──┐  ┌──▼────────────┐
     │ Agente Cartão │  │ Agente Saldo  │
     │  de Crédito   │  │               │
     │ (LLM+Prompt+  │  │ (LLM+Prompt+  │
     │   Memória)    │  │   Memória)    │
     └──────┬────────┘  └───────┬───────┘
            │ MCP               │ MCP
     ┌──────▼────────┐  ┌──────▼────────┐
     │  CreditoBFA   │  │   SaldoBFA    │
     │ ┌────────────┐│  │ ┌────────────┐│
     │ │ Políticas  ││  │ │ Políticas  ││
     │ │ Cache      ││  │ │ Cache      ││
     │ │ Logging    ││  │ │ Logging    ││
     │ │ Auth       ││  │ │ Auth       ││
     │ └────────────┘│  │ └────────────┘│
     └──────┬────────┘  └──────┬────────┘
            │                  │
     ┌──────▼────────┐  ┌─────▼─────────┐
     │ API Cartão de │  │  API de Saldo │
     │   Crédito     │  │               │
     └───────────────┘  └───────────────┘
```

### O que fica encapsulado dentro do BFA

As **ferramentas de domínio** (cartão de crédito, fatura, saldo do cliente, etc.) ficam encapsuladas dentro do BFA. O agente apenas conhece as operações expostas pelo BFA — não os detalhes de implementação.

---

## 4. Responsabilidades do BFA

O BFA é responsável por:

| Responsabilidade | O que faz |
|---|---|
| **Tradução semântica** | Converte dados das APIs de domínio para o formato que o agente espera. |
| **Aplicação de políticas** | Autenticação, autorização, mascaramento de dados sensíveis. |
| **Agregação de dados** | Mescla dados de múltiplas APIs em uma única resposta coesa. |
| **Caching** | Armazena respostas frequentes para reduzir latência e carga. |
| **Logging e Observabilidade** | Centraliza métricas, traces e logs em uma fronteira clara. |
| **Fallback e Resiliência** | Encapsula políticas de degradação (cache, resposta parcial) sem obrigar o agente a decidir como lidar com falhas. |
| **Exposição de Fachada Estável** | Atua como uma Anti-Corruption Layer, protegendo o agente de mudanças nas APIs. |

---

## 5. Princípio Fundamental — Desacoplamento Semântico

O maior poder do BFA é o **desacoplamento semântico**:

- Os **agentes não precisam mais entender os detalhes uns dos outros**
- Eles compartilham **abstrações ou facades** providas pelos BFAs
- Os **contratos são claros e versionados** — cada operação é tipada e contratada
- É possível **evoluir sem quebrar** quem consome

### Separação de responsabilidades

```
┌─────────────────────────────────────────┐
│              AGENTE                      │
│  Responsabilidade: A JORNADA             │
│  - LLM, Prompt, Memória                 │
│  - Decisão de alto nível                 │
│  - Orquestração da experiência           │
│  - NÃO lida com APIs ou tools            │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│                BFA                       │
│  Responsabilidade: O BACKEND             │
│  - Encapsular APIs de domínio            │
│  - Traduzir dados                        │
│  - Aplicar políticas de segurança        │
│  - Cache, fallback, resiliência          │
│  - Expor contratos estáveis via MCP      │
└─────────────────────────────────────────┘
```

> **"A responsabilidade do agente é a jornada — e não o backend."**

---

## 6. Protocolos e Integrações

### MCP (Model Context Protocol)

O **MCP** é o protocolo central que viabiliza o BFA. Em vez de os agentes usarem `tool/function calling` diretamente (padrão LangChain, por exemplo), eles passam a invocar o BFA via protocolo MCP.

**Antes (sem BFA):**
```
Agente → tool/function calling → API de Domínio
```

**Depois (com BFA):**
```
Agente → MCP → BFA → API de Domínio
```

### A2A (Agent-to-Agent)

Com o BFA, é possível integrar comunicação **A2A**, onde cada agente pode invocar seu próprio BFA ou outros agentes para obter dados e orquestrar a comunicação. Isso é viabilizado pelo desacoplamento que o BFA proporciona.

### Multi-Agentes com Supervisor

O BFA se integra naturalmente com o design pattern de **multi-agentes com supervisor** (ex.: LangGraph). O supervisor recebe a requisição e encaminha para subagentes que, por sua vez, têm seus próprios BFAs.

---

## 7. Benefícios do Padrão BFA

| Benefício | Descrição |
|---|---|
| **Desacoplamento semântico** | Agentes não conhecem detalhes de APIs — operam em alto nível. |
| **Contratos versionados e claros** | Cada operação é tipada e contratada, permitindo evolução sem quebra. |
| **Observabilidade centralizada** | Métricas, traces e logs em uma fronteira clara e definida. |
| **Reuso de lógica e composição** | Um agente composer pode orquestrar múltiplos subagentes, cada um com seu BFA, sem replicar lógica. |
| **Fallback e resiliência** | BFAs encapsulam políticas de degradação (cache, resposta parcial). |
| **Simplificação do prompt** | O LLM pode raciocinar em termos de operações de alto nível (ex.: "pegar resumo de cartão e verificar saldo disponível") sem saber como chamar três APIs para montar essa informação. |
| **Segregação em etapas** | Liberdade para quebrar em nós de decisão (ex.: LangGraph). |

---

## 8. Trade-offs

O BFA não é gratuito. Considere os seguintes trade-offs:

| Trade-off | Impacto |
|---|---|
| **Latência extra** | Adiciona mais uma camada na comunicação, o que pode aumentar o tempo de resposta. |
| **Governança de versionamento** | Requer disciplina na gestão de versões dos contratos expostos pelo BFA. |

---

## 9. Pattern Card — Resumo Estruturado

| Campo | Descrição |
|---|---|
| **Nome** | BFA — Back-end for Agents |
| **Problema** | Agentes de IA tendem a se acoplar diretamente às APIs de domínio, dados e prompts, duplicando lógica, criando fragilidade e dificultando a comunicação entre agentes. |
| **Solução** | Introduzir uma camada intermediária (BFA) que encapsula APIs, aplica políticas, traduz dados e expõe operações estáveis via protocolos como MCP. |
| **Benefícios** | Desacoplamento semântico; contratos versionados e claros; observabilidade centralizada; reuso de lógica e composição. |
| **Trade-offs** | Adiciona mais uma camada (latência extra); requer governança de versionamento. |
| **Exemplo** | `Supervisor → Agente de Cartão → CreditBFA → API de Cartão` / `Supervisor → Agente de Saldo → BalanceBFA → API de Saldo` |

---

## 10. Como Implementar o BFA — Guia Prático

### Passo 1: Identifique os domínios

Mapeie as APIs de domínio que seus agentes consomem. Agrupe-as por contexto de negócio (ex.: crédito, saldo, cadastro, atendimento).

### Passo 2: Crie um BFA por domínio

Cada domínio deve ter seu próprio BFA. Ele será responsável por encapsular todas as APIs daquele contexto.

```
CreditoBFA/
├── políticas de autenticação
├── tradução de dados
├── cache
├── logging
├── fallback
└── exposição via MCP
```

### Passo 3: Exponha contratos via MCP

Defina operações tipadas e versionadas que o BFA expõe. Os agentes devem conhecer apenas esses contratos — nunca os detalhes das APIs internas.

### Passo 4: Desacople os agentes

Remova qualquer `tool/function calling` direto das APIs de domínio dentro dos agentes. Substitua por chamadas ao BFA via MCP.

### Passo 5: Implemente resiliência no BFA

O BFA deve lidar com:
- **Cache** para respostas frequentes
- **Fallback** para degradação graciosa
- **Circuit breaker** para proteção contra APIs instáveis
- **Resposta parcial** quando nem todos os dados estão disponíveis

### Passo 6: Centralize observabilidade

Implemente métricas, traces e logs no BFA. Essa é a fronteira clara onde toda a observabilidade do backend do agente deve estar.

### Passo 7: Integre com o Supervisor (opcional)

Se estiver usando um padrão multi-agentes, integre o BFA com o supervisor (ex.: LangGraph) para orquestrar subagentes especializados.

---

## 11. Exemplo Prático — Fluxo Completo

**Pergunta do cliente:** "Posso usar meu limite disponível para pagar uma compra? E qual o impacto no meu saldo?"

```
1. Cliente envia pergunta
2. Supervisor identifica necessidade de consultar crédito e saldo
3. Supervisor chama subagentes:

   AgenteCartãoDeCrédito → CreditoBFA → APIDeCartãoDeCrédito
   AgenteDeSaldo         → SaldoBFA   → APIDeSaldo

4. Cada BFA:
   - Autentica a requisição
   - Chama a API de domínio
   - Traduz a resposta para o formato do agente
   - Aplica cache se necessário
   - Registra métricas e logs

5. Subagentes retornam dados processados ao Supervisor
6. Supervisor agrega as respostas e formula a resposta final
7. Resposta entregue ao cliente
```

---

## 12. Boas Práticas

1. **Cada BFA deve ter um único domínio** — evite BFAs que misturam contextos diferentes.
2. **Contratos devem ser versionados** — permita evolução sem quebrar consumidores.
3. **O agente nunca deve chamar APIs diretamente** — toda comunicação passa pelo BFA.
4. **Use MCP como protocolo de invocação** — padronize a interface entre agente e BFA.
5. **Implemente fallback e cache no BFA** — o agente não deve decidir como lidar com falhas de infraestrutura.
6. **Centralize logging e tracing no BFA** — essa é a fronteira de observabilidade.
7. **Mantenha os prompts simples** — o LLM deve raciocinar em termos de operações de alto nível, não de APIs.
8. **Teste o BFA independentemente** — ele deve ser testável sem depender do agente.

---

## 13. Relação com Outros Padrões

| Padrão | Relação com o BFA |
|---|---|
| **BFF (Back-end for Front-end)** | Inspiração direta. O BFA faz para agentes o que o BFF faz para frontends. |
| **Anti-Corruption Layer** | O BFA atua como uma ACL, protegendo o agente de mudanças nas APIs. |
| **Facade** | O BFA expõe uma fachada simplificada para o agente. |
| **Multi-Agent Supervisor** | O BFA integra-se com supervisores que orquestram múltiplos subagentes. |
| **MCP (Model Context Protocol)** | Protocolo principal de comunicação entre agente e BFA. |
| **A2A (Agent-to-Agent)** | O BFA viabiliza comunicação entre agentes ao desacoplar o backend. |

---

## 14. Futuro do BFA

O BFA é um padrão que ainda está nascendo, mas já demonstrou ganhos claros de desacoplamento e reuso. Recomendações para adoção:

- Experimente isolar o backend dos seus agentes em um BFA
- Use MCP como contrato de invocação
- Compartilhe suas implementações e aprendizados com a comunidade

> *"Assim como o BFF transformou a arquitetura de frontends, o BFA pode transformar a maneira como construímos sistemas de agentes."*  
> — Michael Douglas Barbosa Araujo

---

## Referências

- [O padrão Back-end para Agentes (BFA) — Michael Douglas Barbosa Araujo](https://medium.com/@mdbaraujo/o-padr%C3%A3o-back-end-para-agentes-bfa-a53c1c6d87fb)
- [Repositório do projeto](https://github.com/michaeldouglas/post_medium/tree/main/Python/post_multiagent)
- [BFF — Back-end for Front-end — Phil Calçado](https://philcalcado.com/)
