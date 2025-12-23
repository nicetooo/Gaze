# ADB GUI 🚀

Uma ferramenta de gerenciamento Android poderosa, moderna e independente, construída com **Wails**, **React** e **Ant Design**.

> ✨ **Nota**: Este aplicativo é fruto de puro **vibecoding**.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)

## ✨ Recursos

### 📱 Gerenciamento de Dispositivos
- **Lista Unificada de Dispositivos**: Gerencie dispositivos físicos e sem fio (fusão USB/Wi-Fi) em uma visualização unificada.
- **Conexão Sem Fio**: Conecte-se facilmente via emparelhamento IP/Porta com suporte mDNS.
- **Histórico de Dispositivos**: Acesso rápido a dispositivos offline conectados anteriormente.
- **Pin de Dispositivo**: Fixe seu dispositivo mais usado para que ele permaneça sempre no topo da lista.
- **Atualização Sequencial**: Mecanismo de polling sequencial mais inteligente para uma interface sólida e sem oscilações.

### 📦 Gerenciamento de Apps
- **Controle Total de Pacotes**: Instalar (Arrastar e Soltar), Desinstalar, Ativar, Desativar, Forçar Parada, Limpar Dados.
- **Gerenciamento de APK**: Exportar APKs instalados, Instalação em lote.
- **Filtragem Inteligente**: Busque e filtre por apps do sistema/usuário.
- **Ações Rápidas**: Inicie aplicativos ou pule diretamente para seus logs.

### 🖥️ Espelhamento de Tela (Scrcpy)
- **Alto Desempenho**: Espelhamento de baixa latência impulsionado pelo Scrcpy.
- **Gravação**: Gravação em segundo plano independente com suporte para vários dispositivos simultaneamente e acesso à pasta com um clique.
- **Encaminhamento de Áudio**: Transmita o áudio do dispositivo para o seu computador (Android 11+).
- **Personalização**: Ajuste resolução, taxa de bits, FPS e codec (H.264/H.265).
- **Controle**: Suporte multitoque, Manter acordado, Modo tela desligada.

### 📂 Gerenciamento de Arquivos
- **Explorador Completo**: Navegar, Copiar, Recortar, Colar, Renomear, Excluir e Criar Pastas.
- **Arrastar e Soltar**: Carregue arquivos simplesmente arrastando-os para a janela.
- **Downloads**: Transferência fácil de arquivos do dispositivo para o computador.
- **Visualização**: Abra arquivos diretamente na máquina host usando aplicativos padrão.

### 📜 Logcat Avançado
- **Streaming em Tempo Real**: Visualizador de logs ao vivo com controle de rolagem automática.
- **Filtragem Poderosa**: Filtre por Nível de Log, Tag, PID ou Regex personalizado.
- **Pré-filtragem (Pre-Filter)**: Armazenamento de alto desempenho que guarda apenas logs que correspondem a regras específicas.
- **Centrado no App**: Filtre automaticamente logs para um aplicativo específico.

### 💻 ADB Shell
- **Console Integrado**: Execute comandos ADB brutos diretamente no aplicativo.
- **Histórico de Comandos**: Acesso rápido a comandos executados anteriormente.

### 🔌 Bandeja do Sistema
- **Acesso Rápido**: Controle o espelhamento e veja o status do dispositivo na barra de menu/bandeja do sistema.
- **Pin de Dispositivo**: Fixe seu dispositivo principal para aparecer no topo da lista e no menu da bandeja.
- **Funções da Bandeja**: Acesso direto ao Logcat, Shell e Gerenciador de Arquivos para dispositivos fixados a partir da bandeja.
- **Indicadores de Gravação**: Indicador visual de ponto vermelho na bandeja quando a gravação está ativa.
- **Operação em Segundo Plano**: Mantenha o aplicativo rodando em segundo plano para acesso instantâneo.

---

## 🛠️ Binários Integrados

Esta aplicação é totalmente independente. Ela agrupa:
- **ADB** (Android Debug Bridge)
- Executável **Scrcpy**
- **Scrcpy-server**

Na inicialização, eles são extraídos para um diretório temporário e usados automaticamente. Você não precisa configurar o PATH do seu sistema.

---

## ⚠️ Notas Importantes para Usuários Xiaomi/Poco/Redmi

Para ativar o **controle por toque** no Scrcpy, você deve:
1. Ir em **Opções do Desenvolvedor**.
2. Ativar a **Depuração USB**.
3. Ativar a **Depuração USB (Configurações de segurança)**.
   *(Nota: Isso requer um cartão SIM e login na conta Mi na maioria dos dispositivos Xiaomi).*

---

## 🚀 Primeiros Passos

### Pré-requisitos
- **Go** (v1.21)
- **Node.js** (v18 LTS)
- **Wails CLI** (v2.9.2)
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
  ```

### Desenvolvimento
```bash
wails dev
```

### Build
```bash
wails build
```
A aplicação compilada estará disponível em `build/bin`.

### Release
Este projeto usa GitHub Actions para automatizar builds multiplataforma. Para criar um novo release:
1. Tagueie seu commit: `git tag v1.0.0`
2. Empurre a tag: `git push origin v1.0.0`
A GitHub Action irá buildar automaticamente para macOS, Windows e Linux, e fará o upload dos artefatos para a página de Release.

---

## 🔧 Solução de Problemas

### macOS: "A aplicação está danificada e não pode ser aberta"
Se você baixar o aplicativo do GitHub e vir o erro *"adbGUI.app está danificado e não pode ser aberto"*, isso se deve à quarentena do macOS Gatekeeper.

Para corrigir isso, execute o seguinte comando no seu terminal:
```bash
sudo xattr -cr /path/to/adbGUI.app
```
*(Substitua `/path/to/adbGUI.app` pelo caminho real da aplicação baixada)*

> **Ou compile você mesmo:** Se preferir não contornar o Gatekeeper, você pode facilmente [compilar o aplicativo a partir do código-fonte](#-começando) localmente. Leva apenas alguns minutos!

### Windows: "O Windows protegeu o seu computador"
Se você vir uma janela azul do SmartScreen impedindo a inicialização:
1. Clique em **Mais informações (More info)**.
2. Clique em **Executar assim mesmo (Run anyway)**.

---

## 📄 Licença
Este projeto está licenciado sob a Licença MIT.
