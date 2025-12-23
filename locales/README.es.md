# ADB GUI 🚀

Una herramienta de gestión de Android potente, moderna y autónoma construida con **Wails**, **React** y **Ant Design**.

> ✨ **Nota**: Esta aplicación es fruto del puro **vibecoding**.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)

## ✨ Características

### 📱 Gestión de Dispositivos
- **Lista Unificada de Dispositivos**: Gestiona dispositivos físicos e inalámbricos (fusión USB/Wi-Fi) sin problemas en una vista unificada.
- **Conexión Inalámbrica**: Conéctese sin esfuerzo mediante emparejamiento IP/Puerto con soporte mDNS.
- **Historial de Dispositivos**: Acceso rápido a dispositivos fuera de línea conectados anteriormente.
- **Pin de Dispositivo**: Fije su dispositivo más usado para que permanezca siempre al principio de la lista.
- **Refresco Secuencial**: Mecanismo de sondeo secuencial más inteligente para una interfaz sólida y sin parpadeos.

### 📦 Gestión de Aplicaciones
- **Control Total de Paquetes**: Instalar (Arrastrar y Soltar), Desinstalar, Habilitar, Deshabilitar, Forzar Detención, Borrar Datos.
- **Gestión de APK**: Exportar APKs instalados, Instalación por Lotes.
- **Filtrado Inteligente**: Buscar y filtrar por aplicaciones del sistema/usuario.
- **Acciones Rápidas**: Inicie aplicaciones o salte directamente a sus registros.

### 🖥️ Duplicación de Pantalla (Scrcpy)
- **Alto Rendimiento**: Duplicación de baja latencia impulsada por Scrcpy.
- **Grabación**: Grabación en segundo plano independiente con soporte para múltiples dispositivos simultáneamente y acceso a carpeta con un solo clic.
- **Reenvío de Audio**: Transmita el audio del dispositivo a su computadora (Android 11+).
- **Personalización**: Ajuste resolución, tasa de bits, FPS y códec (H.264/H.265).
- **Control**: Soporte multitáctil, Mantener despierto, Modo pantalla apagada.

### 📂 Gestión de Archivos
- **Explorador con Funciones Completas**: Navegar, Copiar, Cortar, Pegar, Renombrar, Eliminar y Crear Carpetas.
- **Arrastrar y Soltar**: Cargue archivos simplemente arrastrándolos a la ventana.
- **Descargas**: Transferencia de archivos fácil del dispositivo a la computadora.
- **Vista Previa**: Abrir archivos directamente en la máquina host.

### 📜 Logcat Avanzado
- **Transmisión en Tiempo Real**: Visor de registros en vivo con control de desplazamiento automático.
- **Filtrado Potente**: Filtrar por Nivel de Registro, Etiqueta, PID o Regex personalizado.
- **Pre-Filtrado (Pre-Filter)**: Almacenamiento de alto rendimiento que solo guarda registros que coinciden con reglas específicas.
- **Centrado en la Aplicación**: Filtrar automáticamente registros para una aplicación específica.

### 💻 ADB Shell
- **Consola Integrada**: Ejecute comandos ADB sin procesar directamente dentro de la aplicación.
- **Historial de Comandos**: Acceso rápido a comandos ejecutados anteriormente.

### 🔌 Bandeja del Sistema
- **Acceso Rápido**: Controle la duplicación y vea el estado del dispositivo desde la barra de menú/bandeja del sistema.
- **Pin de Dispositivo**: Fije su dispositivo principal para que aparezca al principio de la lista y del menú de la bandeja.
- **Funciones de Bandeja**: Acceso directo a Logcat, Shell y Administrador de Archivos para dispositivos fijados desde la bandeja.
- **Indicadores de Grabación**: Indicador visual de punto rojo en la bandeja cuando la grabación está activa.
- **Operación en Segundo Plano**: Mantenga la aplicación ejecutándose en segundo plano para un acceso instantáneo.

---

## 🛠️ Binarios Integrados

Esta aplicación es totalmente autónoma. Incluye:
- **ADB** (Android Debug Bridge)
- Ejecutable **Scrcpy**
- **Scrcpy-server**

Al inicio, estos se extraen en un directorio temporal y se usan automáticamente. No es necesario configurar el PATH de su sistema.

---

## ⚠️ Notas Importantes para Usuarios de Xiaomi/Poco/Redmi

Para habilitar el **control táctil** en Scrcpy, debe:
1. Ir a **Opciones de Desarrollador**.
2. Habilitar **Depuración USB**.
3. Habilitar **Depuración USB (ajustes de seguridad)**.
   *(Nota: Esto requiere una tarjeta SIM e iniciar sesión en la cuenta Mi en la mayoría de los dispositivos Xiaomi).*

---

## 🚀 Empezando

### Prerrequisitos
- **Go** (v1.21)
- **Node.js** (v18 LTS)
- **Wails CLI** (v2.9.2)
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
  ```

### Desarrollo
```bash
wails dev
```

### Construcción
```bash
wails build
```
La aplicación compilada estará disponible en `build/bin`.

### Lanzamiento
Este proyecto utiliza GitHub Actions para automatizar las construcciones multiplataforma. Para crear un nuevo lanzamiento:
1. Etiquete su commit: `git tag v1.0.0`
2. Empuje la etiqueta: `git push origin v1.0.0`
La GitHub Action construirá automáticamente para macOS, Windows y Linux, y subirá los artefactos a la página de Lanzamientos.

---

## 🔧 Solución de problemas

### macOS: "La aplicación está dañada y no se puede abrir"
Si descargas la aplicación desde GitHub y ves el error *"adbGUI.app está dañada y no se puede abrir"*, esto se debe a la cuarentena de macOS Gatekeeper.

Para solucionar esto, ejecuta el siguiente comando en tu terminal:
```bash
sudo xattr -cr /path/to/adbGUI.app
```
*(Reemplaza `/path/to/adbGUI.app` con la ruta real de tu aplicación descargada)*

> **O compílalo tú mismo:** Si prefieres no eludir Gatekeeper, puedes [compilar la aplicación desde el código fuente](#-empezando) fácilmente de forma local. ¡Solo toma unos minutos!

### Windows: "Windows protegió su PC"
Si ves una ventana azul de SmartScreen impidiendo el inicio:
1. Haz clic en **Más información (More info)**.
2. Haz clic en **Ejecutar de todas formas (Run anyway)**.

---

## 📄 Licencia
Este proyecto está bajo la Licencia MIT.
