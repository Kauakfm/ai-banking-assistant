# Estratégia de RAG — Retrieval Augmented Generation

## Pipeline Completo

```mermaid
graph TB
    subgraph INGESTION["Ingestão (Offline)"]
        Docs[📄 Documentos Base<br/>Políticas · FAQs · Orientações]
        Docs --> Chunker[Chunking]
        Chunker --> Chunks[Chunks]
        Chunks --> EmbModel[Embedding Model]
        EmbModel --> Vectors[Vetores]
        Vectors --> VectorStore[(Vector Store)]
    end

    subgraph RETRIEVAL["Retrieval (Online)"]
        Query([Query do Agente])
        Query --> QueryEmb[Embedding da Query]
        QueryEmb --> Search[Busca Semântica<br/>Similarity Search]
        VectorStore --> Search
        Search --> TopK[Top-K Resultados]
        TopK --> Reranker[Reranker]
        Reranker --> FilteredCtx[Contexto Filtrado]
    end

    subgraph GENERATION["Generation"]
        FilteredCtx --> PromptBuilder[Prompt Builder]
        PromptBuilder --> LLM[LLM]
        LLM --> Response([Resposta Contextualizada])
    end
```

## Estratégia de Chunking

```mermaid
graph LR
    subgraph Input["Documento Original"]
        Doc[Política de Crédito PJ<br/>~5000 palavras]
    end

    subgraph Strategy["Chunking Strategy"]
        Doc --> Splitter[Recursive Text Splitter]
        Splitter --> C1[Chunk 1<br/>~500 tokens]
        Splitter --> C2[Chunk 2<br/>~500 tokens]
        Splitter --> C3[Chunk 3<br/>~500 tokens]
        Splitter --> CN[Chunk N<br/>~500 tokens]
    end

    subgraph Overlap["Overlap"]
        OV[50 tokens overlap<br/>entre chunks adjacentes]
    end

    Strategy --- Overlap
```

| Parâmetro | Valor | Justificativa |
|---|---|---|
| Chunk size | ~500 tokens | Equilíbrio entre contexto e precisão |
| Overlap | ~50 tokens | Evita perda de contexto em bordas |
| Separadores | `\n\n` → `\n` → `. ` → ` ` | Respeita estrutura do documento |
| Metadata | título, seção, categoria | Permite filtragem na busca |

## Modelo de Embedding

```mermaid
graph LR
    Text[Texto] --> Tokenizer[Tokenizer]
    Tokenizer --> Model[Embedding Model<br/>all-MiniLM-L6-v2 /<br/>text-embedding-3-small]
    Model --> Vector[Vetor 384-1536 dims]
```

| Critério | Escolha |
|---|---|
| Modelo local | `all-MiniLM-L6-v2` (384 dims) |
| Modelo cloud | `text-embedding-3-small` (1536 dims) |
| Trade-off | Local = custo zero, cloud = maior qualidade |

## Busca e Recuperação

```mermaid
sequenceDiagram
    participant Agent as Agente
    participant Emb as Embedding Model
    participant VS as Vector Store
    participant RR as Reranker

    Agent->>Emb: encode(query)
    Emb-->>Agent: query_vector
    Agent->>VS: similarity_search(query_vector, top_k=10)
    VS-->>Agent: 10 chunks candidatos
    Agent->>RR: rerank(query, chunks, top_n=3)
    RR-->>Agent: 3 chunks relevantes
    Note over Agent: Injeta no prompt como contexto
```

## Critérios de Recuperação

```mermaid
graph TD
    Query([Query]) --> SimSearch[Similarity Search<br/>Cosine Similarity]
    SimSearch --> Candidates[Top-K Candidatos]
    Candidates --> ScoreFilter{Score > threshold?}
    ScoreFilter -->|Sim| Reranker[Cross-Encoder Reranker]
    ScoreFilter -->|Não| NoContext[Sem contexto RAG<br/>Resposta sem base documental]
    Reranker --> MetadataFilter{Metadata filter<br/>relevante?}
    MetadataFilter -->|Sim| FinalContext[Contexto final para LLM]
    MetadataFilter -->|Não| Discard[Descartado]
```

| Etapa | Threshold | Motivo |
|---|---|---|
| Similarity Score | ≥ 0.7 | Evita chunks muito distantes |
| Reranker Score | Top 3 | Limita contexto no prompt |
| Metadata Match | categoria alinhada | Reduz ruído |

## Evitando Contexto Irrelevante

```mermaid
graph TD
    subgraph Filters["Camadas de Filtragem"]
        F1[1. Similarity Threshold<br/>Descarta score baixo]
        F2[2. Metadata Filter<br/>Categoria / Segmento]
        F3[3. Reranking<br/>Cross-encoder valida relevância]
        F4[4. Max Context Window<br/>Limita tokens no prompt]
    end

    F1 --> F2 --> F3 --> F4
```

## Armazenamento Vetorial

```mermaid
graph TB
    subgraph Local["Desenvolvimento"]
        ChromaDB[(ChromaDB / FAISS)]
    end

    subgraph Prod["Produção"]
        Pinecone[(Pinecone /<br/>OpenSearch Serverless)]
    end

    Local -.->|migração| Prod
```

## Base de Conhecimento (Exemplos)

```mermaid
mindmap
  root((Knowledge Base))
    Políticas de Crédito
      Limites por segmento
      Critérios de aprovação
      Taxas vigentes
    Orientações Financeiras
      Fluxo de caixa
      Capital de giro
      Investimentos PJ
    FAQs
      Abertura de conta
      Operações cambiais
      Antecipação de recebíveis
    Compliance
      KYC / PLD
      Regulamentações BACEN
```
