# Guia de uso da API — Evolution GO Custom

Como integrar o Evolution GO em **outros projetos**, chamando os endpoints direto (sem o painel).
Todos os exemplos usam `curl`, mas é só HTTP + JSON — funciona em qualquer linguagem.

> **Referência completa e interativa:** `http://SEU_SERVIDOR:8080/swagger/index.html`
> (todos os endpoints, campos e schemas). Este guia cobre o caminho principal.

---

## 1. Conceitos: base URL e as duas chaves

- **Base URL:** `http://SEU_SERVIDOR:8080` (a porta padrão é `8080`).
- Toda requisição autentica pelo header **`apikey`**. Existem **duas chaves diferentes**:

| Chave | O que é | Onde usa |
|---|---|---|
| **`GLOBAL_API_KEY`** | A chave "admin" do servidor. Impressa no fim da instalação (ou em `deploy/.env`). | **Só** para **gerenciar instâncias**: criar, listar, deletar, proxy. |
| **Token da instância** | Você **define** ao criar a instância. | **Tudo daquela instância**: conectar, QR, status, **enviar mensagens**, webhooks, grupos, etc. |

Regra prática: **criou/listou instância → usa a `GLOBAL_API_KEY`. Qualquer coisa de uma instância específica (inclusive enviar) → usa o token daquela instância.**

Erro de auth vem como `401 { "error": "not authorized" }`.

---

## 2. Ciclo de uma instância (do zero ao envio)

Uma "instância" = um número de WhatsApp conectado. O fluxo é:

**criar** → **conectar** → **ler o QR** → **checar status** → **enviar**.

### 2.1. Criar a instância (usa `GLOBAL_API_KEY`)

```bash
curl -X POST http://SEU_SERVIDOR:8080/instance/create \
  -H "apikey: GLOBAL_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "instanceId": "11111111-2222-3333-4444-555555555555",
    "name": "comercial-01",
    "token": "MEU_TOKEN_SECRETO_DA_INSTANCIA"
  }'
```

- `instanceId`: um UUID que você gera.
- `name`: nome livre para identificar.
- `token`: **você escolhe** — é este valor que vira a `apikey` de tudo dessa instância. Guarde bem.
- Opcionais: `proxy` (`{ "protocol", "host", "port", "username", "password" }`) e `advancedSettings`.

### 2.2. Conectar e gerar o QR (usa o **token da instância**)

```bash
curl -X POST http://SEU_SERVIDOR:8080/instance/connect \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA" \
  -H "Content-Type: application/json" \
  -d '{ "subscribe": ["ALL"] }'
```

- `subscribe`: eventos que a instância vai emitir (`["ALL"]` = todos).
- `webhookUrl` (opcional): já define o webhook aqui (veja a seção 4).
- `phone` (opcional): se informar o número, o pareamento sai por **código** em vez de QR.

Depois, pegue o QR Code (imagem em base64):

```bash
curl http://SEU_SERVIDOR:8080/instance/qr \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA"
# → { "data": { "Qrcode": "data:image/png;base64,iVBORw0..." } }
```

Renderize esse `data:image` e leia no celular (WhatsApp → Aparelhos conectados). O QR renova a cada ~20s.

### 2.3. Checar o status

```bash
curl http://SEU_SERVIDOR:8080/instance/status \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA"
# → { "data": { "Connected": true, "LoggedIn": true, "Name": "Fulano" } }
```

- `Connected`: socket aberto com o WhatsApp.
- `LoggedIn: true` = número **validado e pronto pra enviar**.

Outras rotas úteis da instância (todas com o token): `POST /instance/disconnect`, `POST /instance/reconnect`, `DELETE /instance/logout`.

---

## 3. Enviar mensagens

Todos os envios são `POST /send/...`, com **`apikey: <token da instância>`** e `Content-Type: application/json`.
O campo `number` aceita um telefone (com DDI, ex.: `5511999998888`) **ou** um JID de grupo (ex.: `123456@g.us`).

### 3.1. Texto

```bash
curl -X POST http://SEU_SERVIDOR:8080/send/text \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA" -H "Content-Type: application/json" \
  -d '{ "number": "5511999998888", "text": "Olá! *Negrito*, _itálico_, ~riscado~." }'
```

### 3.2. Botão (com ou sem foto)

```bash
curl -X POST http://SEU_SERVIDOR:8080/send/button \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA" -H "Content-Type: application/json" \
  -d '{
    "number": "5511999998888",
    "title": "Oferta do dia",
    "description": "Confira as condições",
    "footer": "Sua Loja",
    "imageUrl": "https://.../foto.jpg",
    "buttons": [ { "type": "url", "displayText": "Comprar", "url": "https://exemplo.com" } ]
  }'
```

- `imageUrl` é **opcional** — com ele, a foto vai no topo (renderiza no WhatsApp Web) e o título vira negrito no corpo.
- Tipos de botão: `reply`, `url`, `call`, `copy`, `pix`. Detalhes e regras na seção "Mensagens interativas" do [README](../README.md).

### 3.3. Link com preview (imagem clicável)

```bash
curl -X POST http://SEU_SERVIDOR:8080/send/link \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA" -H "Content-Type: application/json" \
  -d '{
    "number": "5511999998888",
    "text": "Confira nossa oferta 👇\nhttps://sualoja.com/oferta",
    "title": "Oferta especial",
    "description": "Confira as condições exclusivas",
    "imgUrl": "https://.../foto.jpg",
    "largePreview": true
  }'
```

- A **URL precisa estar no `text`** (é dela que sai o preview clicável).
- `imgUrl` vazio → usa o `og:image` da página. `largePreview: true` → imagem **grande**; sem ele → thumbnail pequena.
- Aqui a **imagem inteira é clicável** e abre o link (não tem botão).

### 3.4. Lista, carrossel e mídia

- **Lista:** `POST /send/list` — menu de seleção em seções (`sections[].rows[]`).
- **Carrossel:** `POST /send/carousel` — cards deslizáveis (**mínimo 2** pra renderizar).
- **Mídia:** `POST /send/media` — imagem/vídeo/áudio/documento, por **URL pública ou base64** (campo `url`).
- Outros: `/send/poll`, `/send/sticker`, `/send/location`, `/send/contact`, `/send/status/text`, `/send/status/media`.

Os corpos completos de botão/lista/carrossel estão documentados no [README](../README.md#-mensagens-interativas) e no Swagger.

### 3.5. Resposta de um envio

Sucesso retorna `200` com os dados da mensagem enviada (id, timestamp). Falha retorna status de erro com `{ "error": "..." }`.

---

## 4. Receber mensagens (webhooks)

Para o seu sistema **receber** as mensagens/eventos, cadastre um webhook na instância. Sempre que
chegar um evento (mensagem recebida, recibo de leitura, etc.), o Evolution GO faz um `POST` no seu endpoint.

```bash
curl -X POST http://SEU_SERVIDOR:8080/instance/webhooks/INSTANCE_ID \
  -H "apikey: MEU_TOKEN_SECRETO_DA_INSTANCIA" -H "Content-Type: application/json" \
  -d '{ "url": "https://seu-sistema.com/webhook/evolution" }'
```

- `INSTANCE_ID` é o `instanceId` usado na criação.
- Dá pra ter **vários webhooks por instância** (chame de novo com outra URL).
- Listar: `GET /instance/webhooks/INSTANCE_ID` · Remover: `DELETE .../webhooks/INSTANCE_ID` com `{ "url": "..." }`.
- Alternativa: passar `webhookUrl` já no `POST /instance/connect`.
- Além de webhook, dá pra consumir eventos por **WebSocket** (`/ws?token=GLOBAL_API_KEY&instanceId=...`), RabbitMQ e NATS.

O seu endpoint deve responder `200` rápido. O corpo do POST traz o tipo do evento e os dados da mensagem.

---

## 5. Exemplo ponta-a-ponta (bash)

```bash
BASE="http://SEU_SERVIDOR:8080"
GLOBAL="GLOBAL_API_KEY"
TOKEN="MEU_TOKEN_SECRETO_DA_INSTANCIA"
IID="11111111-2222-3333-4444-555555555555"

# 1) cria a instância (chave global)
curl -s -X POST "$BASE/instance/create" -H "apikey: $GLOBAL" -H "Content-Type: application/json" \
  -d "{\"instanceId\":\"$IID\",\"name\":\"comercial-01\",\"token\":\"$TOKEN\"}"

# 2) conecta (token da instância)
curl -s -X POST "$BASE/instance/connect" -H "apikey: $TOKEN" -H "Content-Type: application/json" \
  -d '{"subscribe":["ALL"]}'

# 3) pega o QR (renderize o data:image e leia no celular)
curl -s "$BASE/instance/qr" -H "apikey: $TOKEN"

# 4) espera ficar LoggedIn:true
curl -s "$BASE/instance/status" -H "apikey: $TOKEN"

# 5) envia
curl -s -X POST "$BASE/send/text" -H "apikey: $TOKEN" -H "Content-Type: application/json" \
  -d '{"number":"5511999998888","text":"Funcionou! 🚀"}'
```

---

## 6. Referência de endpoints (resumo)

| Grupo | Auth | Endpoints |
|---|---|---|
| **Gerenciar instância** | `GLOBAL_API_KEY` | `POST /instance/create`, `GET /instance/all`, `GET /instance/info/{id}`, `DELETE /instance/delete/{id}`, `POST\|DELETE /instance/proxy/{id}` |
| **Operar instância** | token da instância | `POST /instance/connect`, `GET /instance/qr`, `GET /instance/status`, `POST /instance/pair`, `POST /instance/disconnect`, `DELETE /instance/logout`, `*/instance/webhooks/{id}` |
| **Enviar** | token da instância | `POST /send/text\|link\|media\|button\|list\|carousel\|poll\|sticker\|location\|contact\|status/*` |
| **Interagir** | token da instância | `/message/*`, `/chat/*`, `/group/*`, `/user/*`, `/label/*`, `/newsletter/*`, `/polls/*` |

Lista completa com todos os campos: **`/swagger/index.html`** no seu servidor.
