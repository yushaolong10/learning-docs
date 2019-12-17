1.第三章 第六节 变量作用域
ZE使用_zend_execute_data来存储某个单独的op_array（每个函数都会生成单独的op_array)执行过程中所需要的信息，它的结构如下：

函数中的局部变量就存储在_zend_execute_data的symbol_table中，在执行当前函数的op_array时，全局zend_executor_globals中的 *** active_symbol_table会指向当前_zend_execute_data中的*** *symbol_table。

2.疑问：
why很多操作，对于opline->op2.u.EA.type 很热衷做最后的类型判断
```php
static inline HashTable *zend_get_target_symbol_table(const zend_op *opline, const temp_variable *Ts, int type, const zval *variable TSRMLS_DC){ 
		switch (opline->op2.u.EA.type) {                ... //  省略
               case ZEND_FETCH_GLOBAL:
                case ZEND_FETCH_GLOBAL_LOCK:
                        return &EG(symbol_table);                        break;
               ...  //  省略        
    	}        
        return NULL;
    }
```

3.对于zend函数的设定
```php
typedef union _zend_function {
    zend_uchar type;    /* 如用户自定义则为 #define ZEND_USER_FUNCTION 2
                            MUST be the first element of this struct! */
 
    struct {
        zend_uchar type;  /* never used */
        char *function_name;    //函数名称
        zend_class_entry *scope; //函数所在的类作用域
        zend_uint fn_flags;     // 作为方法时的访问类型等，如ZEND_ACC_STATIC等  
        union _zend_function *prototype; //函数原型
        zend_uint num_args;     //参数数目
        zend_uint required_num_args; //需要的参数数目
        zend_arg_info *arg_info;  //参数信息指针
        zend_bool pass_rest_by_reference;
        unsigned char return_reference;  //返回值 
    } common;
 
    zend_op_array op_array;   //函数中的操作,一个函数多个opcode
    zend_internal_function internal_function;  
} zend_function;
```
函数执行的数据源是内部变量:op_array
zend_function的结构中的op_array存储了该函数中所有的操作，当函数被调用时，ZE就会将这个op_array中的opline一条条顺次执行，并将最后的返回值返回。从VLD扩展中查看的关于函数的信息可以看出，函数的定义和执行是分开的，一个函数可以作为一个独立的运行单元而存在。

```php
typedef struct _zend_internal_function {    /* Common elements */
    zend_uchar type;    
    char * function_name;    
    zend_class_entry *scope;
    zend_uint fn_flags;    
    union _zend_function *prototype;
    zend_uint num_args;    
    zend_uint required_num_args;
    zend_arg_info *arg_info;    
    zend_bool pass_rest_by_reference;    
    unsigned char return_reference;    /* END of common elements */ 
    void (*handler)(INTERNAL_FUNCTION_PARAMETERS);    
    struct _zend_module_entry *module;//模块标识
} zend_internal_function;
```
函数有不同的类型:用户函数，内置函数，eval函数，加载函数，对于内置函数，用到其内置参数module做标识
最常见的操作是在模块初始化时，ZE会遍历每个载入的扩展模块，然后将模块中function_entry中指明的每一个函数(module->functions)，创建一个zend_internal_function结构， 并将其type设置为ZEND_INTERNAL_FUNCTION，将这个结构填入全局的函数表(HashTable结构）; 函数设置及注册过程见 Zend/zend_API.c文件中的 zend_register_functions函数。这个函数除了处理函数，也处理类的方法，包括那些魔术方法。

4.函数传参
```php
php内核对函数传参的处理：先根据入参的数量以*数组的形式*分配内存空间，opcode数据结构arg_info指向该内存的指针，然后对该数组中每一个数据结构元素进行赋值
CG(active_op_array)->arg_info = erealloc(CG(active_op_array)->arg_info,
        sizeof(zend_arg_info)*(CG(active_op_array)->num_args));
cur_arg_info = &CG(active_op_array)->arg_info[CG(active_op_array)->num_args-1];
cur_arg_info->name = estrndup(varname->u.constant.value.str.val,
        varname->u.constant.value.str.len);
cur_arg_info->name_len = varname->u.constant.value.str.len;
cur_arg_info->array_type_hint = 0;
cur_arg_info->allow_null = 1;
cur_arg_info->pass_by_reference = pass_by_reference;
cur_arg_info->class_name = NULL;
cur_arg_info->class_name_len = 0;
```

整个参数的传递是通过给中间代码的arg_info字段执行赋值操作完成。关键点是在arg_info字段。arg_info字段的结构如下：
```php
typedef struct _zend_arg_info {
    const char *name;   /* 参数的名称*/
    zend_uint name_len;     /* 参数名称的长度*/
    const char *class_name; /* 类名 */
    zend_uint class_name_len;   /* 类名长度*/
    zend_bool array_type_hint;  /* 数组类型提示 */
    zend_bool allow_null;   /* 是否允许为NULL　*/
    zend_bool pass_by_reference;    /*　是否引用传递 */
    zend_bool return_reference; 
    int required_num_args;  
} zend_arg_info;
```

5.匿名函数
函数在内核中保存在 EG(function_table)
匿名函数是以闭包对象的__invoke方法来实现
use 是通过函数内部静态数据结构进行传值，推测：进行内存拷贝(常规赋值)或者指针拷贝(引用赋值)
`closure->func.op_array.static_variables`

6.类与对象
类在内核中保存在EG(class_table)
```php
class Tipi
{
	public static function t()
	{
		echo 1;
	}
}
Tipi::t();
```
opcode如下：
```php
number of ops:  8
compiled vars:  none
line     # *  op                           fetch          ext  return  operands
---------------------------------------------------------------------------------
   2     0  >   EXT_STMT
         1      NOP
   8     2      EXT_STMT
         3      ZEND_INIT_STATIC_METHOD_CALL                             'Tipi','t'
         4      EXT_FCALL_BEGIN
         5      DO_FCALL_BY_NAME                              0
         6      EXT_FCALL_END
   9     7    > RETURN                                                   1
 
branch: #  0; line:     2-    9; sop:     0; eop:     7
path #1: 0,
Class Tipi:
Function t:
Finding entry points
Branch analysis from position: 0
```
从以上的内容可以看出整个静态成员方法的调用是一个先查找方法，再调用的过程。而对于调用操作，对应的中间代码为 ZEND_INIT_STATIC_METHOD_CALL。由于（类名）和（方法名）都是常量，于是我们可以知道中间代码对应的函数是ZEND_INIT_STATIC_METHOD_CALL_SPEC_CONST_CONST_HANDLER。在这个函数中，它会首先调用zend_fetch_class函数，通过类名在EG(class_table)中查找类，然后再执行静态方法的获取方法。

7.注意在bison中，解析者占据一个位,如下`$9`是值 `method_body`，由于 `{ zend_do_begin_function_declaration(&$2, &$4, 1, $3.op_type, &$1 TSRMLS_CC); }`占据了`$5`
```php
class_statement:
		variable_modifiers { CG(access_type) = Z_LVAL($1.u.constant); } class_variable_declaration ';'
	|	class_constant_declaration ';'
	|	trait_use_statement
	|	method_modifiers function is_reference T_STRING { zend_do_begin_function_declaration(&$2, &$4, 1, $3.op_type, &$1 TSRMLS_CC); }
		'(' parameter_list ')'
		method_body { zend_do_abstract_method(&$4, &$1, &$9 TSRMLS_CC); zend_do_end_function_declaration(&$2 TSRMLS_CC); }
;
```

8.关于接口的实现
```php
interface_entry:
    T_INTERFACE { 
    $$.u.opline_num = CG(zend_lineno);
    $$.u.EA.type = ZEND_ACC_INTERFACE;
    }
;

new_class_entry->ce_flags |= class_token->u.EA.type;//注意: 1.bison进行语法解析后，进行传值， 2. |= 是用或进行补位，这样可以与之前的ce_flags参数互补


```

9.关于抽象类
```php
static int ZEND_FASTCALL  ZEND_NEW_SPEC_HANDLER(ZEND_OPCODE_HANDLER_ARGS)
{
    zend_op *opline = EX(opline);    
    zval *object_zval;
    zend_function *constructor; 
    if (EX_T(opline->op1.u.var).class_entry->ce_flags & (ZEND_ACC_INTERFACE|ZEND_ACC_IMPLICIT_ABSTRACT_CLASS|ZEND_ACC_EXPLICIT_ABSTRACT_CLASS)) {
        char *class_type; 
        if (EX_T(opline->op1.u.var).class_entry->ce_flags & ZEND_ACC_INTERFACE) {
            class_type = "interface";
        } else {
        	class_type = "abstract class";
        }
        zend_error_noreturn(E_ERROR, "Cannot instantiate %s %s", class_type,  EX_T(opline->op1.u.var).class_entry->name);//这里注意EX_T(opline->op1.u.var)的含义，指的是variable变量
	}
	// ...
}

```

10.对于class_scope结构赋值
```php
	EX(object) = object_zval;
    EX(fbc) = constructor;
    EX(called_scope) = EX_T(opline->op1.u.var).class_entry;//scope赋值ce结构
    ZEND_VM_NEXT_OPCODE();
```

11.对于作用域的EG(scope)的认识
```php
zend_class_entry *zend_fetch_class(const char *class_name, uint class_name_len, int fetch_type TSRMLS_DC)
{
    zend_class_entry **pce;
    int use_autoload = (fetch_type & ZEND_FETCH_CLASS_NO_AUTOLOAD) == 0;
    int silent       = (fetch_type & ZEND_FETCH_CLASS_SILENT) != 0;
 
    fetch_type &= ZEND_FETCH_CLASS_MASK;
 
check_fetch_type:
    switch (fetch_type) {
        case ZEND_FETCH_CLASS_SELF:
            if (!EG(scope)) {
                zend_error(E_ERROR, "Cannot access self:: when no class scope is active");
            }
            return EG(scope);
        case ZEND_FETCH_CLASS_PARENT:
            if (!EG(scope)) {
                zend_error(E_ERROR, "Cannot access parent:: when no class scope is active");
            }
            if (!EG(scope)->parent) {
                zend_error(E_ERROR, "Cannot access parent:: when current class scope has no parent");
            }
            return EG(scope)->parent;
        case ZEND_FETCH_CLASS_STATIC:
            if (!EG(called_scope)) {
                zend_error(E_ERROR, "Cannot access static:: when no class scope is active");
            }
            return EG(called_scope);
        case ZEND_FETCH_CLASS_AUTO: {
                fetch_type = zend_get_class_fetch_type(class_name, class_name_len);
                if (fetch_type!=ZEND_FETCH_CLASS_DEFAULT) {
                    goto check_fetch_type;
                }
            }
            break;
    }
 
    if (zend_lookup_class_ex(class_name, class_name_len, use_autoload, &pce TSRMLS_CC) == FAILURE) {
        if (use_autoload) {
            if (!silent && !EG(exception)) {
                if (fetch_type == ZEND_FETCH_CLASS_INTERFACE) {
                    zend_error(E_ERROR, "Interface '%s' not found", class_name);
                } else {
                    zend_error(E_ERROR, "Class '%s' not found", class_name);
                }
                }
            }
        }
        return NULL;
    }
    return *pce;
}
```
从上面函数就能看出端倪了，当需要获取self类的时候，则将EG(scope)类返回，而EG(scope)指向的正是当前类。如果时parent类的话则从去EG(scope)->parent也就是当前类的父类，而static获取的时EG(called_scope)，分别说说EG宏的这几个字段，前面已经介绍过EG宏，它可以展开为如下这个结构体:
```php

struct _zend_executor_globals {
    // ...
    zend_class_entry *scope;
    zend_class_entry *called_scope; /* Scope of the calling class */
    // ...
    zend_objects_store objects_store;//对象池
}
 
struct _zend_class_entry {
    char type;
    char *name;
    zend_uint name_length;
    struct _zend_class_entry *parent;
}
#define struct _zend_class_entry zend_class_entry
```

这里针对对象，我们引入一个新的概念--对象池。我们将PHP内核在运行中存储所有对象的列表称之为对象池，即EG(objects_store)。这个对象池的作用是存储PHP中间代码运行阶段所有生成的对象，

12.对于命名空间的操作，是在编译过程中指定的。所以很多初始化的操作都是以CG()
 `tmp.u.constant = *CG(current_namespace);`
 
 13.spl部分库介绍
 和双向链表一样， SplFixedArray类实现了Iterator，ArrayAccess和Countable接口。从而可以直接用foreach遍历整个链表，可以以数组的方式访问对象，调用count方法获取数组的长度。在获取数组元素值时，如果所传递的不是整数的下标，则抛出RuntimeException: Index invalid or out of range异常。与获取元素末端，在设置数组元素时，如果所传递的不是整数的下标，会抛出RuntimeExceptione异常。如果所设置的下标已经存在的值，则会先释放旧值的空间，然后将新的值指向旧值的空间。当通过unset函数翻译数组中的元素时，如果参数指定的下标存在值，则释放值所占的空间，并设置为NULL。

SplObjectStorage类实现了对象存储映射表，应用于需要唯一标识多个对象的存储场景。在PHP5.3.0之前仅能存储对象，之后可以针对每个对象添加一条对应的数据。 SplObjectStorage类的数据存储依赖于PHP的HashTable实现，与传统的使用数组和spl_object_hash函数生成数组key相比，其直接使用HashTable的实现在性能上有较大的优势。有一些奇怪的是，在PHP手册中，SplObjectStorage类放在数据结构目录下。但是他的实现和观察者模式的接口放在同一个文件（ext/spl/spl_observer.c）。实际上他们并没有直接的关系。


14.php内存管理： cow  (copy_on_write)，写时复制
```php
<?php
$foo['love'] = 1;
$bar  = &$foo['love'];//php内核对数组引用操作，产生复制分离
$tipi = $foo;
$tipi['love'] = '2';
echo $foo['love'];//2
```
这个例子最后会输出 2 ， 大家会非常惊讶于$tipi怎么会影响到$foo, 是因为$bar变量的引用操作，将$foo['love']污染变成了引用，从而Zend没有对$tipi['love']的修改产生内存的复制分离。

15.php内存,鸟哥链接：http://www.laruence.com/2011/11/09/2277.html
对于zend_mm梳理：
- 1.获取小块内存small-bucket，宏如下：
```php
#define ZEND_MM_SMALL_FREE_BUCKET(heap, index) \
(zend_mm_free_block*) ((char*)&heap->free_buckets[index * 2] + \ //加char *指针强制转换
    sizeof(zend_mm_free_block*) * 2 - \ //略过两个结构体指针
    sizeof(zend_mm_small_free_block))
```
仔细看这个宏实现，发现在它的计算过程是取free_buckets列表的偶数位的内存地址加上两个指针的内存大小并减去zend_mm_small_free_block结构所占空间的大小。而zend_mm_free_block结构和zend_mm_small_free_block结构的差距在于两个指针。据此计算过程可知，ZEND_MM_SMALL_FREE_BUCKET宏会获取free_buckets列表 index对应双向链表的第一个zend_mm_free_block的prev_free_block指向的位置。 free_buckets的计算仅仅与prev_free_block指针和next_free_block指针相关，所以free_buckets列表也仅仅需要存储这两个指针。

16.php内存: 新的垃圾回收
分为两块：
对于引用计数：
ref_count == 0 ,则直接回收
ref_count == 1, 对于该数据进行垃圾鉴定： 打上标签 ：黑色，灰色，紫色，白色
```php
GC_WHITE 白色表示垃圾 
GC_PURPLE 紫色表示已放入缓冲区 
GC_GREY 灰色表示已经进行了一次refcount的减一操作 
GC_BLACK 黑色是默认颜色，正常 
```

17.第七章第三节 中间代码的执行
18.第八章第三节 php的线程安全
19.foreach实现