# Cliente Stop-and-Wait en Go para Tareas 2 y 3 de CC4303 (2022-1)

## Instrucciones

Para utilizar este cliente, usted deberá primero compilarlo utilizando el compilador de Go.
Se recomienda seguir las siguientes instrucciones _dentro_ de la máquina virtual para la Tarea 3.

1. Instale Go siguiendo las las instrucciones en [este enlace](https://go.dev/doc/install). En resumen (para Linux):
   1. Borre posibles instalaciones previas de Go: `sudo rm -rf /usr/local/go`.
   2. Descargue el compilador:

      ```bash
      $ cd ~/Downloads
      $ sudo apt install wget -y
      ...
      $ wget https://go.dev/dl/go1.18.3.linux-amd64.tar.gz
      ```

   3. Descomprima e instale el compilador: `sudo tar -C /usr/local -xzf ~/Downloads/go1.18.3.linux-amd64.tar.gz`
   4. Use el siguiente comando para que la consola pueda encontrar el comando `go`: `echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile`
      (copie el comando textualmente, y fíjese en las comillas simples alrededor de `'export PATH=$PATH:/usr/local/go/bin'`).
   5. Reinicie la máquina virtual.
   6. Verifique su instalación de Go mediante el siguiente comando:
   
      ``` bash
      $ go version
      go version go1.18.3 linux/amd64
      ```
2. Ya instalado `go`, puede fácilmente obtener y compilar el código en este repositorio.
   1. Para descargar, compilar, e instalar el código: `go install -tags netgo github.com/molguin92/CC4303-T2-2022@v1.0.3`.
      Fíjese bien en la opción `-tags netgo`; es necesaria para que su código corra sin problemas dentro de Kathará.
   2. El binario compilado quedará en `~/go/bin/`, y el código fuente en `~/go/pkg/`.
   3. Si desea modificar el código, puede volver a compilarlo usando `go build -tags netgo github.com/molguin92/CC4303-T2-2022`.

## Utilizar el Cliente

Puede obtener instrucciones simples de cómo utilizar el cliente especificando la opción `--help` al ejecutar el binario:

```bash
$ ~/go/bin/CC4303-T2-2022 --help
Usage:
  CC4303-T2-2022 TIMEOUT_MS DATAGRAM_SIZE_BYTES INPUT_FILE OUTPUT_FILE HOST PORT [flags]

Flags:
  -h, --help          help for CC4303-T2-2022
  -r, --record-rtts   Record RTTs; samples will be output as CSV files ./recvRTTs.csv and ./sendRTTs.csv in the current directory.
  -s, --record-stats   Record stats for total time, dropped packets, and dropped ACKS. Will be stored as a JSON file ./stats.json in the current directory.
```

El cliente sigue la misma interfaz especificada en el enunciado de la Tarea 2, con una opción adicional.
Si al invocar el ejecutable usted además agrega la opción `-r` (o `--record-rtts`), el cliente escribirá dos archivos CSV en la actual carpeta con los RTTs de envío y recepción, los cuales puede utilizar para responder las preguntas de la T3.
Si agrega la opción `-s` (`--record-stats`), el cliente creará un archivo `stats.json` con información sobre el tiempo total que tomó enviar y recibir el archivo (en segundos), y sobre la cantidad de paquetes/ACKS perdidos.

## Licencia de Uso

Copyright 2022 Manuel Olguín Muñoz.
El código está licenciado bajo una licencia Apache v2, ver [LICENSE](LICENSE) para más detalles.
